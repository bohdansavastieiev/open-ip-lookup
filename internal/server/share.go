package server

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	sharepkg "github.com/bohdansavastieiev/open-ip-lookup/internal/share"
)

type createShareRequest struct {
	Input string `json:"input"`
}

type createShareResponse struct {
	Path string `json:"path"`
}

type resolveShareRequest struct {
	Bearer string `json:"bearer"`
}

type resolveShareResponse struct {
	Input string `json:"input"`
}

func (s *Server) handleShare(w http.ResponseWriter, r *http.Request) {
	startedAt := time.Now()
	clientIP := clientIPText(r)
	if !s.shareCreateLimiter.allow(clientIP) {
		s.logger.Info("share create rate limited", slog.String("client_ip", clientIP))
		writeJSONError(w, http.StatusTooManyRequests, "too many share requests")
		return
	}

	var req createShareRequest
	if err := decodeJSONRequest(w, r, maxLookupBodyBytes, &req); err != nil {
		if errors.Is(err, errRequestTooLarge) {
			writeJSONError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}

		writeJSONError(w, http.StatusBadRequest, "invalid share request")
		return
	}

	created, err := s.service.CreateShare(r.Context(), req.Input)
	if errors.Is(err, sharepkg.ErrNoIPs) {
		writeJSONError(w, http.StatusBadRequest, "no IP addresses found")
		return
	}
	if err != nil {
		s.logger.Error("create share", slog.Any("err", err))
		writeJSONError(w, http.StatusInternalServerError, "share creation failed")
		return
	}

	s.logger.Info(
		"share created",
		slog.Int64("share_id", created.ID),
		slog.String("client_ip", clientIP),
		slog.Time("expires_at", created.ExpiresAt),
		slog.Duration("duration", time.Since(startedAt)),
	)

	writeJSON(w, http.StatusOK, createShareResponse{Path: "/#s=" + created.Bearer})
}

func (s *Server) handleShareResolve(w http.ResponseWriter, r *http.Request) {
	startedAt := time.Now()
	clientIP := clientIPText(r)
	if !s.shareResolveLimiter.allow(clientIP) {
		s.logger.Info("share resolve rate limited", slog.String("client_ip", clientIP))
		writeJSONError(w, http.StatusTooManyRequests, "too many share resolve requests")
		return
	}

	var req resolveShareRequest
	if err := decodeJSONRequest(w, r, maxShareResolveBodyBytes, &req); err != nil {
		if errors.Is(err, errRequestTooLarge) {
			writeJSONError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}

		writeJSONError(w, http.StatusBadRequest, "invalid share resolve request")
		return
	}
	if req.Bearer == "" {
		writeJSONError(w, http.StatusBadRequest, "invalid share resolve request")
		return
	}

	resolved, err := s.service.ResolveShare(r.Context(), req.Bearer)
	if errors.Is(err, sharepkg.ErrNotFound) {
		writeJSONError(w, http.StatusNotFound, "share not found")
		return
	}
	if err != nil {
		s.logger.Error("resolve share", slog.Any("err", err))
		writeJSONError(w, http.StatusInternalServerError, "share resolve failed")
		return
	}

	s.logger.Info(
		"share resolved",
		slog.Int64("share_id", resolved.ID),
		slog.String("client_ip", clientIP),
		slog.Int("visit_count", resolved.VisitCount),
		slog.Time("expires_at", resolved.ExpiresAt),
		slog.Duration("duration", time.Since(startedAt)),
	)

	writeJSON(w, http.StatusOK, resolveShareResponse{Input: resolved.Input})
}
