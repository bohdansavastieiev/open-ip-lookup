package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/share"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShareCreate_ReturnsSharePath(t *testing.T) {
	svc := &fakeLookupService{
		createdShare: share.Created{ID: 42, Bearer: "test-bearer"},
	}
	srv := newTestServer(t, svc)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/shares",
		strings.NewReader(`{"input":"1.1.1.1\n1.1.1.1"}`),
	)
	res := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(res, req)

	require.True(t, svc.createShareCalled)
	assert.Equal(t, "1.1.1.1\n1.1.1.1", svc.createShareInput)
	assert.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, noStoreCacheControl, res.Header().Get("Cache-Control"))

	var got createShareResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &got))
	assert.Equal(t, "/#s=test-bearer", got.Path)
}

func TestShareCreate_RejectsOversizedBodyBeforeService(t *testing.T) {
	svc := &fakeLookupService{}
	srv := newTestServer(t, svc)
	body := `{"input":"` + strings.Repeat("x", maxLookupBodyBytes) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/shares", strings.NewReader(body))
	res := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(res, req)

	assert.False(t, svc.createShareCalled)
	assert.Equal(t, http.StatusRequestEntityTooLarge, res.Code)
}

func TestShareCreate_RejectsInvalidJSON(t *testing.T) {
	svc := &fakeLookupService{}
	srv := newTestServer(t, svc)
	req := httptest.NewRequest(http.MethodPost, "/api/shares", strings.NewReader("{"))
	res := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(res, req)

	assert.False(t, svc.createShareCalled)
	assert.Equal(t, http.StatusBadRequest, res.Code)
}

func TestShareCreate_RejectsTrailingJSON(t *testing.T) {
	svc := &fakeLookupService{}
	srv := newTestServer(t, svc)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/shares",
		strings.NewReader(`{"input":"1.1.1.1"}{"input":"2.2.2.2"}`),
	)
	res := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(res, req)

	assert.False(t, svc.createShareCalled)
	assert.Equal(t, http.StatusBadRequest, res.Code)
}

func TestShareCreate_RejectsInputWithoutIPs(t *testing.T) {
	svc := &fakeLookupService{createShareErr: share.ErrNoIPs}
	srv := newTestServer(t, svc)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/shares",
		strings.NewReader(`{"input":"no addresses"}`),
	)
	res := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(res, req)

	require.True(t, svc.createShareCalled)
	assert.Equal(t, http.StatusBadRequest, res.Code)
}

func TestShareCreate_RateLimitsByClientIP(t *testing.T) {
	svc := &fakeLookupService{
		createdShare: share.Created{ID: 42, Bearer: "test-bearer"},
	}
	srv := newTestServer(t, svc)
	srv.shareCreateLimiter = newRateLimiter(1, time.Minute)

	first := newCreateShareRequest("203.0.113.9")
	firstRes := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(firstRes, first)
	require.Equal(t, http.StatusOK, firstRes.Code)

	svc.createShareCalled = false
	second := newCreateShareRequest("203.0.113.9")
	secondRes := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(secondRes, second)

	assert.False(t, svc.createShareCalled)
	assert.Equal(t, http.StatusTooManyRequests, secondRes.Code)
}

func TestShareResolve_ReturnsInput(t *testing.T) {
	svc := &fakeLookupService{
		resolvedShare: share.Resolved{
			ID:         42,
			Input:      "1.1.1.1\n1.1.1.1",
			VisitCount: 3,
		},
	}
	srv := newTestServer(t, svc)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/shares/resolve",
		strings.NewReader(`{"bearer":"test-bearer"}`),
	)
	res := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(res, req)

	require.True(t, svc.resolveCalled)
	assert.Equal(t, "test-bearer", svc.resolveBearer)
	assert.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, noStoreCacheControl, res.Header().Get("Cache-Control"))

	var got resolveShareResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &got))
	assert.Equal(t, "1.1.1.1\n1.1.1.1", got.Input)
}

func TestShareResolve_RateLimitsByClientIP(t *testing.T) {
	svc := &fakeLookupService{
		resolvedShare: share.Resolved{ID: 42, Input: "1.1.1.1", VisitCount: 1},
	}
	srv := newTestServer(t, svc)
	srv.shareResolveLimiter = newRateLimiter(1, time.Minute)

	first := newResolveShareRequest("203.0.113.9")
	firstRes := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(firstRes, first)
	require.Equal(t, http.StatusOK, firstRes.Code)

	svc.resolveCalled = false
	second := newResolveShareRequest("203.0.113.9")
	secondRes := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(secondRes, second)

	assert.False(t, svc.resolveCalled)
	assert.Equal(t, http.StatusTooManyRequests, secondRes.Code)
}

func TestShareResolve_RejectsInvalidJSON(t *testing.T) {
	svc := &fakeLookupService{}
	srv := newTestServer(t, svc)
	req := httptest.NewRequest(http.MethodPost, "/api/shares/resolve", strings.NewReader("{"))
	res := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(res, req)

	assert.False(t, svc.resolveCalled)
	assert.Equal(t, http.StatusBadRequest, res.Code)
}

func TestShareResolve_RejectsTrailingJSON(t *testing.T) {
	svc := &fakeLookupService{}
	srv := newTestServer(t, svc)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/shares/resolve",
		strings.NewReader(`{"bearer":"test-bearer"}{"bearer":"other"}`),
	)
	res := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(res, req)

	assert.False(t, svc.resolveCalled)
	assert.Equal(t, http.StatusBadRequest, res.Code)
}

func TestShareResolve_RejectsOversizedBodyBeforeService(t *testing.T) {
	svc := &fakeLookupService{}
	srv := newTestServer(t, svc)
	body := `{"bearer":"` + strings.Repeat("x", maxShareResolveBodyBytes) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/shares/resolve", strings.NewReader(body))
	res := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(res, req)

	assert.False(t, svc.resolveCalled)
	assert.Equal(t, http.StatusRequestEntityTooLarge, res.Code)
}

func TestShareResolve_ReturnsNotFound(t *testing.T) {
	svc := &fakeLookupService{resolveShareErr: share.ErrNotFound}
	srv := newTestServer(t, svc)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/shares/resolve",
		strings.NewReader(`{"bearer":"missing"}`),
	)
	res := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(res, req)

	require.True(t, svc.resolveCalled)
	assert.Equal(t, http.StatusNotFound, res.Code)
}

func newCreateShareRequest(clientIP string) *http.Request {
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/shares",
		strings.NewReader(`{"input":"1.1.1.1"}`),
	)
	req.RemoteAddr = "172.18.0.2:12345"
	req.Header.Set("CF-Connecting-IP", clientIP)
	return req
}

func newResolveShareRequest(clientIP string) *http.Request {
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/shares/resolve",
		strings.NewReader(`{"bearer":"test-bearer"}`),
	)
	req.RemoteAddr = "172.18.0.2:12345"
	req.Header.Set("CF-Connecting-IP", clientIP)
	return req
}
