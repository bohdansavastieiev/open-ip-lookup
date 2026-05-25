// Package server wires the app's HTTP transport (routes, handlers, and embedded UI assets).
package server

import (
	"context"
	"embed"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"net/netip"
	"time"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/config"
	"github.com/bohdansavastieiev/open-ip-lookup/internal/report"
	"github.com/bohdansavastieiev/open-ip-lookup/internal/share"
)

const (
	maxLookupBodyBytes       = 1024 * 1024
	maxShareResolveBodyBytes = 4 * 1024
	shareCreateRateLimitMax  = 20
	shareCreateRateWindow    = 10 * time.Minute
	shareResolveRateLimitMax = 120
	shareResolveRateWindow   = 10 * time.Minute
)

const contentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self'; " +
	"style-src 'self'; " +
	"base-uri 'none'; " +
	"frame-ancestors 'none'; " +
	"form-action 'self'; " +
	"object-src 'none'"

const (
	noStoreCacheControl    = "no-store"
	staticCacheControl     = "no-cache"
	staticFlagCacheControl = "public, max-age=604800"
)

//go:embed templates/*
var templates embed.FS

//go:embed static/*
var staticFiles embed.FS

type Server struct {
	templates  *template.Template
	service    service
	logger     *slog.Logger
	httpServer *http.Server

	shareCreateLimiter  *rateLimiter
	shareResolveLimiter *rateLimiter
}

type service interface {
	HasMaxMind() bool
	Report(string) *report.Report
	LookupIP(netip.Addr) report.IPInfo
	CreateShare(context.Context, string) (share.Created, error)
	ResolveShare(context.Context, string) (share.Resolved, error)
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
		templates:           tmps,
		service:             service,
		logger:              logger,
		shareCreateLimiter:  newRateLimiter(shareCreateRateLimitMax, shareCreateRateWindow),
		shareResolveLimiter: newRateLimiter(shareResolveRateLimitMax, shareResolveRateWindow),
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
	mux.Handle("POST /api/lookup", cacheControl(noStoreCacheControl, http.HandlerFunc(s.handleLookup)))
	mux.Handle("GET /api/client-ip", cacheControl(noStoreCacheControl, http.HandlerFunc(s.handleClientIPLookup)))
	mux.Handle("POST /api/shares", cacheControl(noStoreCacheControl, http.HandlerFunc(s.handleShare)))
	mux.Handle(
		"POST /api/shares/resolve",
		cacheControl(noStoreCacheControl, http.HandlerFunc(s.handleShareResolve)),
	)
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
