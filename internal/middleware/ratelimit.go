package middleware

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// entry holds the rate limiter for a single IP.
type entry struct {
	limiter  *rate.Limiter
	mu       sync.Mutex
	lastSeen time.Time
}

func (e *entry) touch() {
	e.mu.Lock()
	e.lastSeen = time.Now()
	e.mu.Unlock()
}

func (e *entry) seen() time.Time {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.lastSeen
}

type rateLimitError struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// RateLimiter is an IP-based rate limiter with automatic cleanup.
type RateLimiter struct {
	mu    sync.Map
	rps   float64
	burst int
}

// NewRateLimiter creates a RateLimiter and starts a cleanup goroutine.
// The cleanup goroutine runs every 5 minutes and removes IPs not seen for 10+ minutes.
// It respects ctx cancellation via select.
func NewRateLimiter(ctx context.Context, rps float64, burst int) *RateLimiter {
	rl := &RateLimiter{
		rps:   rps,
		burst: burst,
	}
	go rl.startCleanup(ctx)
	return rl
}

func (rl *RateLimiter) startCleanup(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.cleanup()
		case <-ctx.Done():
			return
		}
	}
}

func (rl *RateLimiter) cleanup() {
	cutoff := time.Now().Add(-10 * time.Minute)
	rl.mu.Range(func(key, value any) bool {
		e, ok := value.(*entry)
		if ok && e.seen().Before(cutoff) {
			rl.mu.Delete(key)
		}
		return true
	})
}

// Middleware returns an http.Handler middleware that applies IP-based rate limiting.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return rl.MiddlewareWithProxies("", next)
}

// MiddlewareWithProxies returns an http.Handler middleware with trusted proxies support.
func (rl *RateLimiter) MiddlewareWithProxies(trustedProxies string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := getIP(r, trustedProxies)
		e := rl.getEntry(ip)

		if !e.limiter.Allow() {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(rateLimitError{
				Error: "rate limit exceeded",
				Code:  "ERR_RATE_LIMIT",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) getEntry(ip string) *entry {
	if v, ok := rl.mu.Load(ip); ok {
		e := v.(*entry)
		e.touch()
		return e
	}
	e := &entry{
		limiter: rate.NewLimiter(rate.Limit(rl.rps), rl.burst),
	}
	e.touch()
	actual, _ := rl.mu.LoadOrStore(ip, e)
	result := actual.(*entry)
	if result != e {
		result.touch()
	}
	return result
}

// getIP extracts the client IP from the request.
// If X-Forwarded-For is present and RemoteAddr IP is a trusted proxy, uses the first XFF IP.
func getIP(r *http.Request, trustedProxies string) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}

	if trustedProxies == "" {
		return host
	}

	xff := r.Header.Get("X-Forwarded-For")
	if xff == "" {
		return host
	}

	if isTrustedProxy(host, trustedProxies) {
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}

	return host
}

func isTrustedProxy(ip, trustedProxies string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}

	for _, proxy := range strings.Split(trustedProxies, ",") {
		proxy = strings.TrimSpace(proxy)
		if proxy == "" {
			continue
		}

		if strings.Contains(proxy, "/") {
			_, network, err := net.ParseCIDR(proxy)
			if err == nil && network.Contains(parsed) {
				return true
			}
		} else {
			if net.ParseIP(proxy).Equal(parsed) {
				return true
			}
		}
	}

	return false
}
