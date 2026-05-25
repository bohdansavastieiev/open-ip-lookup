package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookup_ReturnsServiceReportForSubmittedInput(t *testing.T) {
	svc := &fakeLookupService{
		report: &report.Report{
			Stats:   report.Stats{Total: 2, Unique: 1, Reported: 1},
			Entries: []report.Entry{{IPInfo: report.IPInfo{IP: "1.1.1.1", Kind: "Routable"}}},
		},
	}
	srv := newTestServer(t, svc)

	form := url.Values{"input": {"1.1.1.1\n1.1.1.1"}}
	req := httptest.NewRequest(http.MethodPost, "/api/lookup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(res, req)

	require.True(t, svc.reportCalled)
	assert.Equal(t, "1.1.1.1\n1.1.1.1", svc.reportInput)
	assert.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, noStoreCacheControl, res.Header().Get("Cache-Control"))

	var got report.Report
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &got))
	assert.Equal(t, *svc.report, got)
}

func TestLookup_RejectsOversizedBodyBeforeService(t *testing.T) {
	svc := &fakeLookupService{report: &report.Report{}}
	srv := newTestServer(t, svc)

	body := "input=" + strings.Repeat("x", maxLookupBodyBytes)
	req := httptest.NewRequest(http.MethodPost, "/api/lookup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(res, req)

	assert.False(t, svc.reportCalled)
	assert.Equal(t, http.StatusRequestEntityTooLarge, res.Code)
}

func TestClientIPLookup_ReturnsIPInfoForCloudflareIP(t *testing.T) {
	svc := &fakeLookupService{
		ipInfo: report.IPInfo{IP: "203.0.113.10", Kind: "Routable"},
	}
	srv := newTestServer(t, svc)
	req := httptest.NewRequest(http.MethodGet, "/api/client-ip", nil)
	req.RemoteAddr = "172.18.0.2:12345"
	req.Header.Set("CF-Connecting-IP", "203.0.113.10")
	res := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(res, req)

	require.True(t, svc.lookupCalled)
	assert.Equal(t, netip.MustParseAddr("203.0.113.10"), svc.lookupIP)
	assert.False(t, svc.reportCalled)
	assert.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, noStoreCacheControl, res.Header().Get("Cache-Control"))

	var got report.IPInfo
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &got))
	assert.Equal(t, svc.ipInfo, got)
}

func TestClientIPLookup_IgnoresCloudflareIPFromUntrustedPeer(t *testing.T) {
	svc := &fakeLookupService{ipInfo: report.IPInfo{IP: "198.51.100.3", Kind: "Routable"}}
	srv := newTestServer(t, svc)
	req := httptest.NewRequest(http.MethodGet, "/api/client-ip", nil)
	req.RemoteAddr = "198.51.100.3:12345"
	req.Header.Set("CF-Connecting-IP", "203.0.113.10")
	res := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(res, req)

	require.True(t, svc.lookupCalled)
	assert.Equal(t, netip.MustParseAddr("198.51.100.3"), svc.lookupIP)
	assert.Equal(t, http.StatusOK, res.Code)
}

func TestClientIPLookup_FallsBackToRemoteAddress(t *testing.T) {
	svc := &fakeLookupService{ipInfo: report.IPInfo{IP: "198.51.100.3", Kind: "Routable"}}
	srv := newTestServer(t, svc)
	req := httptest.NewRequest(http.MethodGet, "/api/client-ip", nil)
	req.RemoteAddr = "198.51.100.3:12345"
	res := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(res, req)

	require.True(t, svc.lookupCalled)
	assert.Equal(t, netip.MustParseAddr("198.51.100.3"), svc.lookupIP)
	assert.False(t, svc.reportCalled)
	assert.Equal(t, http.StatusOK, res.Code)
}

func TestClientIPLookup_RejectsInvalidDetectedIP(t *testing.T) {
	svc := &fakeLookupService{}
	srv := newTestServer(t, svc)
	req := httptest.NewRequest(http.MethodGet, "/api/client-ip", nil)
	req.RemoteAddr = "172.18.0.2:12345"
	req.Header.Set("CF-Connecting-IP", "invalid")
	res := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(res, req)

	assert.False(t, svc.lookupCalled)
	assert.False(t, svc.reportCalled)
	assert.Equal(t, http.StatusBadRequest, res.Code)
}
