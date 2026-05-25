package server

import (
	"errors"
	"log/slog"
	"net/http"
	"time"
)

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
		slog.String("client_ip", clientIPText(r)),
		slog.Duration("duration", time.Since(startedAt)),
		slog.Int("total", rpt.Stats.Total),
		slog.Int("unique", rpt.Stats.Unique),
		slog.Int("reported", rpt.Stats.Reported),
	)

	writeJSON(w, http.StatusOK, rpt)
}

func (s *Server) handleClientIPLookup(w http.ResponseWriter, r *http.Request) {
	ip, err := clientIP(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid client IP")
		return
	}

	writeJSON(w, http.StatusOK, s.service.LookupIP(ip))
}
