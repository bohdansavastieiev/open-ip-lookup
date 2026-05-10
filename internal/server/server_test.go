package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/config"
	"github.com/bohdansavastieiev/open-ip-lookup/internal/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeLookupService struct {
	hasMaxMind bool
	report     *report.Report
	raw        string
	called     bool
}

func (s *fakeLookupService) HasMaxMind() bool {
	return s.hasMaxMind
}

func (s *fakeLookupService) Report(raw string) *report.Report {
	s.called = true
	s.raw = raw
	return s.report
}

func TestLookup_ReturnsServiceReportForSubmittedInput(t *testing.T) {
	svc := &fakeLookupService{
		report: &report.Report{
			Stats:   report.Stats{Total: 2, Unique: 1, Reported: 1},
			Entries: []report.Entry{{IP: "1.1.1.1", Kind: "Routable"}},
		},
	}
	srv := newTestServer(t, svc)

	form := url.Values{"input": {"1.1.1.1\n1.1.1.1"}}
	req := httptest.NewRequest(http.MethodPost, "/lookup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(res, req)

	require.True(t, svc.called)
	assert.Equal(t, "1.1.1.1\n1.1.1.1", svc.raw)
	assert.Equal(t, http.StatusOK, res.Code)

	var got report.Report
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &got))
	assert.Equal(t, *svc.report, got)
}

func TestLookup_RejectsOversizedBodyBeforeService(t *testing.T) {
	svc := &fakeLookupService{report: &report.Report{}}
	srv := newTestServer(t, svc)

	body := "input=" + strings.Repeat("x", maxLookupBodyBytes)
	req := httptest.NewRequest(http.MethodPost, "/lookup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(res, req)

	assert.False(t, svc.called)
	assert.Equal(t, http.StatusRequestEntityTooLarge, res.Code)
}

func TestHome_RendersMaxMindAttributionWhenAvailable(t *testing.T) {
	tests := []struct {
		name       string
		hasMaxMind bool
		want       bool
	}{
		{name: "with MaxMind", hasMaxMind: true, want: true},
		{name: "without MaxMind", hasMaxMind: false, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeLookupService{hasMaxMind: tt.hasMaxMind}
			srv := newTestServer(t, svc)
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			res := httptest.NewRecorder()

			srv.httpServer.Handler.ServeHTTP(res, req)

			require.Equal(t, http.StatusOK, res.Code)
			assert.Equal(t, tt.want, strings.Contains(res.Body.String(), "GeoLite2 data"))
		})
	}
}

func TestHome_RendersLookupBodyLimit(t *testing.T) {
	svc := &fakeLookupService{}
	srv := newTestServer(t, svc)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	res := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(res, req)

	require.Equal(t, http.StatusOK, res.Code)
	assert.Contains(t, res.Body.String(), fmt.Sprintf(`data-max-body-bytes="%v"`, maxLookupBodyBytes))
}

func TestHealth_ReturnsOK(t *testing.T) {
	svc := &fakeLookupService{}
	srv := newTestServer(t, svc)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	res := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(res, req)

	assert.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, "text/plain; charset=utf-8", res.Header().Get("Content-Type"))
	assert.Equal(t, "ok\n", res.Body.String())
}

func TestSecurityHeaders_AreSet(t *testing.T) {
	svc := &fakeLookupService{}
	srv := newTestServer(t, svc)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	res := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(res, req)

	assert.Equal(t, contentSecurityPolicy, res.Header().Get("Content-Security-Policy"))
	assert.Equal(t, "nosniff", res.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "strict-origin-when-cross-origin", res.Header().Get("Referrer-Policy"))
	assert.Equal(
		t,
		"geolocation=(), microphone=(), camera=()",
		res.Header().Get("Permissions-Policy"),
	)
}

func newTestServer(t *testing.T, svc *fakeLookupService) *Server {
	t.Helper()

	srv, err := New(makeTestServerConfig(), svc, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)

	return srv
}

func makeTestServerConfig() config.ServerConfig {
	return config.ServerConfig{
		Addr:                     ":8080",
		ReadHeaderTimeoutSeconds: 5,
		ReadTimeoutSeconds:       15,
		WriteTimeoutSeconds:      15,
		IdleTimeoutSeconds:       60,
		ShutdownTimeoutSeconds:   5,
	}
}
