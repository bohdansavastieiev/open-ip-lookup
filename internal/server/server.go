// Package server wires the app's HTTP transport (routes, handlers, and embedded UI assets).
package server

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"html/template"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/config"
	"github.com/bohdansavastieiev/open-ip-lookup/internal/report"
)

const maxLookupBodyBytes = 1024 * 1024

const contentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self'; " +
	"style-src 'self'; " +
	"base-uri 'none'; " +
	"frame-ancestors 'none'; " +
	"form-action 'self'; " +
	"object-src 'none'"

const noStoreCacheControl = "no-store"
const staticCacheControl = "no-cache"
const staticFlagCacheControl = "public, max-age=604800"

//go:embed templates/*
var templates embed.FS

//go:embed static/*
var staticFiles embed.FS

type Server struct {
	templates  *template.Template
	service    service
	logger     *slog.Logger
	httpServer *http.Server
}

type service interface {
	HasMaxMind() bool
	Report(string) *report.Report
}

type templateData struct {
	HasMaxMind         bool
	MaxLookupBodyBytes int
}

type noDirectoryListingFS struct {
	fs.FS
}

func New(cfg config.ServerConfig, service service, logger *slog.Logger) (*Server, error) {
	tmps, err := template.New("").ParseFS(templates, "templates/*.html")
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	s := &Server{
		templates: tmps,
		service:   service,
		logger:    logger,
	}
	if err := s.setupRoutes(mux); err != nil {
		return nil, err
	}

	s.httpServer = &http.Server{
		Addr:              cfg.Addr,
		Handler:           securityHeaders(mux),
		ReadHeaderTimeout: time.Duration(cfg.ReadHeaderTimeoutSeconds) * time.Second,
		ReadTimeout:       time.Duration(cfg.ReadTimeoutSeconds) * time.Second,
		WriteTimeout:      time.Duration(cfg.WriteTimeoutSeconds) * time.Second,
		IdleTimeout:       time.Duration(cfg.IdleTimeoutSeconds) * time.Second,
	}

	return s, nil
}

func (s *Server) setupRoutes(mux *http.ServeMux) error {
	staticRoot, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return err
	}

	staticHandler := http.StripPrefix(
		"/static/",
		http.FileServer(http.FS(noDirectoryListingFS{FS: staticRoot})),
	)
	mux.Handle("GET /{$}", cacheControl(noStoreCacheControl, http.HandlerFunc(s.handleHome)))
	mux.Handle("GET /healthz", cacheControl(noStoreCacheControl, http.HandlerFunc(s.handleHealth)))
	mux.Handle("POST /lookup", cacheControl(noStoreCacheControl, http.HandlerFunc(s.handleLookup)))
	mux.Handle("GET /static/flags/", cacheControl(staticFlagCacheControl, staticHandler))
	mux.Handle("GET /static/", cacheControl(staticCacheControl, staticHandler))

	return nil
}

func (f noDirectoryListingFS) Open(name string) (fs.File, error) {
	file, err := f.FS.Open(name)
	if err != nil {
		return nil, err
	}

	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if info.IsDir() {
		_ = file.Close()
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	return file, nil
}

func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	data := templateData{
		HasMaxMind:         s.service.HasMaxMind(),
		MaxLookupBodyBytes: maxLookupBodyBytes,
	}
	if err := s.templates.ExecuteTemplate(w, "index.html", data); err != nil {
		s.logger.Error("render home", "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok\n"))
}

func (s *Server) handleLookup(w http.ResponseWriter, r *http.Request) {
	startedAt := time.Now()
	r.Body = http.MaxBytesReader(w, r.Body, maxLookupBodyBytes)

	if err := r.ParseForm(); err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			writeJSONError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}

		writeJSONError(w, http.StatusBadRequest, "invalid lookup request")
		return
	}

	rpt := s.service.Report(r.Form.Get("input"))
	s.logger.Info(
		"lookup completed",
		slog.String("client_ip", cloudflareClientIP(r)),
		slog.Duration("duration", time.Since(startedAt)),
		slog.Int("total", rpt.Stats.Total),
		slog.Int("unique", rpt.Stats.Unique),
		slog.Int("reported", rpt.Stats.Reported),
	)

	writeJSON(w, http.StatusOK, rpt)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", contentSecurityPolicy)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		next.ServeHTTP(w, r)
	})
}

func cacheControl(value string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", value)
		next.ServeHTTP(w, r)
	})
}

func cloudflareClientIP(r *http.Request) string {
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return ip
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
