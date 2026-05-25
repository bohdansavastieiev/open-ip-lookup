package server

import "net/http"

type templateData struct {
	HasMaxMind         bool
	MaxLookupBodyBytes int
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
