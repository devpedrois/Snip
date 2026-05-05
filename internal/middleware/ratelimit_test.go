package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestRateLimiter_BurstExceeded(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rl := NewRateLimiter(ctx, 1.0/60.0, 3)
	handler := rl.Middleware(okHandler())

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code, "request %d should be 200", i+1)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.Equal(t, "60", rr.Header().Get("Retry-After"))

	var body map[string]string
	err := json.NewDecoder(rr.Body).Decode(&body)
	require.NoError(t, err)
	assert.Equal(t, "ERR_RATE_LIMIT", body["code"])
}

func TestRateLimiter_DifferentIPsIndependent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rl := NewRateLimiter(ctx, 1.0, 1)
	handler := rl.Middleware(okHandler())

	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.RemoteAddr = "10.0.0.1:1111"
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)
	assert.Equal(t, http.StatusOK, rr1.Code)

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "10.0.0.2:2222"
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusOK, rr2.Code)
}

func TestRateLimiter_CleanupRemovesStaleEntries(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rl := NewRateLimiter(ctx, 1.0, 1)

	old := &entry{
		limiter:  nil,
		lastSeen: time.Now().Add(-15 * time.Minute),
	}
	recent := &entry{
		limiter:  nil,
		lastSeen: time.Now(),
	}

	rl.mu.Store("old-ip", old)
	rl.mu.Store("recent-ip", recent)

	rl.cleanup()

	_, oldExists := rl.mu.Load("old-ip")
	_, recentExists := rl.mu.Load("recent-ip")

	assert.False(t, oldExists, "stale entry should be removed")
	assert.True(t, recentExists, "recent entry should remain")
}

func TestRateLimiter_RetryAfterHeader(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rl := NewRateLimiter(ctx, 1.0/60.0, 1)
	handler := rl.Middleware(okHandler())

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.3:9999"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code == http.StatusTooManyRequests {
			assert.Equal(t, "60", rr.Header().Get("Retry-After"))
			return
		}
	}

	t.Fatal("expected a 429 response to verify Retry-After header")
}

func TestGetIP_TrustedProxyUsesXFF(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:0"
	req.Header.Set("X-Forwarded-For", "203.0.113.5")

	ip := getIP(req, "127.0.0.1")
	assert.Equal(t, "203.0.113.5", ip)
}

func TestGetIP_UntrustedProxyIgnoresXFF(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.5")

	ip := getIP(req, "127.0.0.1")
	assert.Equal(t, "10.0.0.1", ip)
}

func TestGetIP_CIDRTrustedProxy(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.50:8080"
	req.Header.Set("X-Forwarded-For", "203.0.113.5")

	ip := getIP(req, "10.0.0.0/8")
	assert.Equal(t, "203.0.113.5", ip)
}
