package server

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/config"
	"github.com/bohdansavastieiev/open-ip-lookup/internal/report"
	"github.com/bohdansavastieiev/open-ip-lookup/internal/share"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeLookupService struct {
	hasMaxMind        bool
	report            *report.Report
	ipInfo            report.IPInfo
	createdShare      share.Created
	resolvedShare     share.Resolved
	createShareErr    error
	resolveShareErr   error
	reportInput       string
	reportCalled      bool
	lookupIP          netip.Addr
	lookupCalled      bool
	createShareInput  string
	createShareCalled bool
	resolveBearer     string
	resolveCalled     bool
}

func (s *fakeLookupService) HasMaxMind() bool {
	return s.hasMaxMind
}

func (s *fakeLookupService) Report(raw string) *report.Report {
	s.reportCalled = true
	s.reportInput = raw
	return s.report
}

func (s *fakeLookupService) LookupIP(ip netip.Addr) report.IPInfo {
	s.lookupCalled = true
	s.lookupIP = ip
	return s.ipInfo
}

func (s *fakeLookupService) CreateShare(_ context.Context, raw string) (share.Created, error) {
	s.createShareCalled = true
	s.createShareInput = raw
	return s.createdShare, s.createShareErr
}

func (s *fakeLookupService) ResolveShare(_ context.Context, bearer string) (share.Resolved, error) {
	s.resolveCalled = true
	s.resolveBearer = bearer
	return s.resolvedShare, s.resolveShareErr
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

func TestHome_UsesNoStoreCache(t *testing.T) {
	svc := &fakeLookupService{}
	srv := newTestServer(t, svc)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	res := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(res, req)

	assert.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, noStoreCacheControl, res.Header().Get("Cache-Control"))
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

func TestStatic_ServesFlagAsset(t *testing.T) {
	svc := &fakeLookupService{}
	srv := newTestServer(t, svc)
	req := httptest.NewRequest(http.MethodGet, "/static/flags/4x3/ua.svg", nil)
	res := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(res, req)

	assert.Equal(t, http.StatusOK, res.Code)
	assert.Contains(t, res.Header().Get("Content-Type"), "image/svg+xml")
	assert.Equal(t, staticFlagCacheControl, res.Header().Get("Cache-Control"))
	assert.Contains(t, res.Body.String(), "<svg")
}

func TestStatic_ServesAppAssetWithoutLongCache(t *testing.T) {
	svc := &fakeLookupService{}
	srv := newTestServer(t, svc)
	req := httptest.NewRequest(http.MethodGet, "/static/app.js", nil)
	res := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(res, req)

	assert.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, staticCacheControl, res.Header().Get("Cache-Control"))
}

func TestStatic_DoesNotListDirectories(t *testing.T) {
	tests := []string{
		"/static/",
		"/static/flags/",
		"/static/flags/4x3/",
	}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			svc := &fakeLookupService{}
			srv := newTestServer(t, svc)
			req := httptest.NewRequest(http.MethodGet, path, nil)
			res := httptest.NewRecorder()

			srv.httpServer.Handler.ServeHTTP(res, req)

			assert.Equal(t, http.StatusNotFound, res.Code)
			assert.NotContains(t, res.Body.String(), "<pre>")
		})
	}
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
