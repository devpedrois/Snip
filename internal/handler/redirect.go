package handler

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/devpedrois/snip/internal/analytics"
	"github.com/devpedrois/snip/internal/domain"
	"github.com/devpedrois/snip/internal/middleware"
	"github.com/devpedrois/snip/internal/service"
)

var hashPattern = regexp.MustCompile(`^[0-9a-zA-Z]{7}$`)

type ClickSubmitter interface {
	Submit(event analytics.ClickEvent)
}

// CacheDeleter is used to invalidate poisoned cache entries.
type CacheDeleter interface {
	Delete(ctx context.Context, hash string) error
}

type RedirectHandler struct {
	svc            service.RedirectorService
	dispatcher     ClickSubmitter
	cache          CacheDeleter
	trustedProxies string
}

func NewRedirectHandler(svc service.RedirectorService, dispatcher ClickSubmitter) *RedirectHandler {
	return &RedirectHandler{svc: svc, dispatcher: dispatcher}
}

// NewRedirectHandlerWithCache creates a handler that can evict poisoned cache entries.
func NewRedirectHandlerWithCache(svc service.RedirectorService, dispatcher ClickSubmitter, cache CacheDeleter) *RedirectHandler {
	return &RedirectHandler{svc: svc, dispatcher: dispatcher, cache: cache}
}

// SetTrustedProxies configures the IP ranges whose X-Forwarded-For header is trusted.
func (h *RedirectHandler) SetTrustedProxies(proxies string) {
	h.trustedProxies = proxies
}

func (h *RedirectHandler) Handle(w http.ResponseWriter, r *http.Request) {
	hash := chi.URLParam(r, "hash")

	if !hashPattern.MatchString(hash) {
		WriteError(w, http.StatusNotFound, "url not found", "ERR_NOT_FOUND")
		return
	}

	u, err := h.svc.Resolve(r.Context(), hash)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrURLNotFound):
			WriteError(w, http.StatusNotFound, "url not found", "ERR_NOT_FOUND")
		case errors.Is(err, domain.ErrURLExpired):
			WriteError(w, http.StatusGone, "url expired", "ERR_URL_EXPIRED")
		case errors.Is(err, domain.ErrURLMalicious):
			report := ""
			if u != nil && u.VTPermalink != nil {
				report = *u.VTPermalink
			}
			WriteJSON(w, http.StatusForbidden, MaliciousErrorResponse{
				Error:  "this url has been flagged as malicious and is no longer available",
				Code:   "ERR_URL_MALICIOUS",
				Report: report,
			})
		default:
			slog.Error("redirect: internal error", "hash", hash, "err", err)
			WriteError(w, http.StatusInternalServerError, "internal server error", "ERR_INTERNAL")
		}
		return
	}

	cleaned := sanitizeRedirectURL(u.OriginalURL)

	parsed, parseErr := url.Parse(cleaned)
	validScheme := parseErr == nil && parsed != nil && (parsed.Scheme == "http" || parsed.Scheme == "https")
	if !validScheme {
		scheme := ""
		if parsed != nil {
			scheme = parsed.Scheme
		}
		slog.Error("redirect: cache poisoning detected", "hash", hash, "url_scheme", scheme)
		if h.cache != nil {
			if delErr := h.cache.Delete(r.Context(), hash); delErr != nil {
				slog.Warn("redirect: failed to evict poisoned cache key", "hash", hash, "err", delErr)
			}
		}
		WriteError(w, http.StatusInternalServerError, "internal server error", "ERR_INTERNAL")
		return
	}

	rawIP := clientIP(r, h.trustedProxies)
	h.dispatcher.Submit(analytics.ClickEvent{
		URLID:      u.ID,
		AccessedAt: time.Now().UTC(),
		UserAgent:  middleware.SanitizeLogValue(r.Header.Get("User-Agent")),
		IP:         analytics.AnonymizeIP(rawIP),
	})

	w.Header().Set("Cache-Control", "private, max-age=0, no-store")
	w.Header().Set("Location", cleaned)
	w.WriteHeader(http.StatusMovedPermanently)
}

func sanitizeRedirectURL(raw string) string {
	return strings.NewReplacer("\r", "", "\n", "", "\t", "", "\x00", "").Replace(raw)
}

func clientIP(r *http.Request, trustedProxies string) string {
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
			_, cidr, err := net.ParseCIDR(proxy)
			if err == nil && cidr.Contains(parsed) {
				return true
			}
		} else if net.ParseIP(proxy).Equal(parsed) {
			return true
		}
	}
	return false
}
