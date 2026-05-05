package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/devpedrois/snip/internal/middleware"
)

func TestSecurityHeaders(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.SecurityHeaders(next)

	tests := []struct {
		name   string
		header string
		want   string
	}{
		{"X-Content-Type-Options", "X-Content-Type-Options", "nosniff"},
		{"X-Frame-Options", "X-Frame-Options", "DENY"},
		{"X-XSS-Protection", "X-XSS-Protection", "1; mode=block"},
		{"Referrer-Policy", "Referrer-Policy", "no-referrer"},
		{"Content-Security-Policy", "Content-Security-Policy", "default-src 'none'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			assert.Equal(t, tt.want, rr.Header().Get(tt.header))
		})
	}
}

func TestNewCORS(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	corsMiddleware := middleware.NewCORS("https://example.com")
	handler := corsMiddleware(next)

	t.Run("allowed origin is reflected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/", nil)
		req.Header.Set("Origin", "https://example.com")
		req.Header.Set("Access-Control-Request-Method", "GET")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, "https://example.com", rr.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("disallowed origin gets no ACAO header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/", nil)
		req.Header.Set("Origin", "https://evil.com")
		req.Header.Set("Access-Control-Request-Method", "GET")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Empty(t, rr.Header().Get("Access-Control-Allow-Origin"))
	})
}
