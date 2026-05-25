package server

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
)

var (
	errInvalidJSONRequest = errors.New("invalid json request")
	errRequestTooLarge    = errors.New("request body too large")
)

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func decodeJSONRequest(w http.ResponseWriter, r *http.Request, maxBytes int64, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return classifyJSONDecodeError(err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return classifyJSONDecodeError(err)
	}
	return nil
}

func classifyJSONDecodeError(err error) error {
	if err == nil {
		return errInvalidJSONRequest
	}
	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		return errRequestTooLarge
	}
	return errInvalidJSONRequest
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

func clientIP(r *http.Request) (netip.Addr, error) {
	return netip.ParseAddr(clientIPText(r))
}

func clientIPText(r *http.Request) string {
	remoteIP := remoteIPText(r)
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" && trustedForwarder(remoteIP) {
		return ip
	}

	return remoteIP
}

func remoteIPText(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func trustedForwarder(ipText string) bool {
	ip, err := netip.ParseAddr(ipText)
	if err != nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate()
}
