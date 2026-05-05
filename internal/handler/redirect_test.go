package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devpedrois/snip/internal/analytics"
	"github.com/devpedrois/snip/internal/domain"
	"github.com/devpedrois/snip/internal/handler"
)

type mockRedirectorService struct {
	resolveFn func(ctx context.Context, hash string) (*domain.URL, error)
}

func (m *mockRedirectorService) Resolve(ctx context.Context, hash string) (*domain.URL, error) {
	return m.resolveFn(ctx, hash)
}

type mockClickSubmitter struct {
	submitFn func(event analytics.ClickEvent)
}

func (m *mockClickSubmitter) Submit(event analytics.ClickEvent) {
	if m.submitFn != nil {
		m.submitFn(event)
	}
}

func buildRedirectRequest(hash string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/"+hash, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("hash", hash)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestRedirectHandler_Handle(t *testing.T) {
	tests := []struct {
		name           string
		hash           string
		resolveURL     *domain.URL
		resolveErr     error
		expectedStatus int
		expectedLoc    string
	}{
		{
			name: "success: 301 with Location and Cache-Control",
			hash: "abc1234",
			resolveURL: &domain.URL{
				ID:          1,
				Hash:        "abc1234",
				OriginalURL: "https://example.com",
				VTStatus:    "clean",
			},
			expectedStatus: http.StatusMovedPermanently,
			expectedLoc:    "https://example.com",
		},
		{
			name:           "not found: 404",
			hash:           "zzzzzzz",
			resolveErr:     domain.ErrURLNotFound,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "expired: 410",
			hash:           "expired",
			resolveErr:     domain.ErrURLExpired,
			expectedStatus: http.StatusGone,
		},
		{
			name:           "internal error: 500",
			hash:           "dbErr12",
			resolveErr:     errors.New("unexpected db error"),
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "invalid hash format: 404 without calling service",
			hash:           "!!!",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "hash too short: 404 without calling service",
			hash:           "abc",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "malicious url: 403",
			hash:           "mal1234",
			resolveErr:     domain.ErrURLMalicious,
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &mockRedirectorService{
				resolveFn: func(_ context.Context, hash string) (*domain.URL, error) {
					if tc.resolveURL == nil && tc.resolveErr == nil {
						t.Fatal("service should not be called for invalid hash")
					}
					return tc.resolveURL, tc.resolveErr
				},
			}

			h := handler.NewRedirectHandler(svc, &mockClickSubmitter{})
			w := httptest.NewRecorder()
			req := buildRedirectRequest(tc.hash)

			h.Handle(w, req)

			resp := w.Result()
			require.Equal(t, tc.expectedStatus, resp.StatusCode)

			if tc.expectedLoc != "" {
				assert.Equal(t, tc.expectedLoc, resp.Header.Get("Location"))
			}

			if tc.expectedStatus == http.StatusMovedPermanently {
				assert.Equal(t, "private, max-age=0, no-store", resp.Header.Get("Cache-Control"))
			}
		})
	}
}

func TestRedirectHandler_MaliciousURL_403WithPermalink(t *testing.T) {
	permalink := "https://virustotal.com/gui/url/abc123"

	svc := &mockRedirectorService{
		resolveFn: func(_ context.Context, _ string) (*domain.URL, error) {
			return &domain.URL{
				Hash:        "mal1234",
				VTStatus:    "malicious",
				VTPermalink: &permalink,
			}, domain.ErrURLMalicious
		},
	}

	h := handler.NewRedirectHandler(svc, &mockClickSubmitter{})
	w := httptest.NewRecorder()
	req := buildRedirectRequest("mal1234")

	h.Handle(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "ERR_URL_MALICIOUS")
	assert.Contains(t, body, "malicious")
	assert.Contains(t, body, permalink)
}

func TestRedirectHandler_MaliciousURL_403NoPermalink(t *testing.T) {
	svc := &mockRedirectorService{
		resolveFn: func(_ context.Context, _ string) (*domain.URL, error) {
			return &domain.URL{Hash: "mal1234", VTStatus: "malicious"}, domain.ErrURLMalicious
		},
	}

	h := handler.NewRedirectHandler(svc, &mockClickSubmitter{})
	w := httptest.NewRecorder()
	req := buildRedirectRequest("mal1234")

	h.Handle(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "ERR_URL_MALICIOUS")
}

func TestRedirectHandler_SubmitsClickEventOnSuccess(t *testing.T) {
	submitted := make(chan analytics.ClickEvent, 1)

	svc := &mockRedirectorService{
		resolveFn: func(_ context.Context, _ string) (*domain.URL, error) {
			return &domain.URL{
				ID:          42,
				Hash:        "abc1234",
				OriginalURL: "https://example.com",
				VTStatus:    "clean",
			}, nil
		},
	}
	dispatcher := &mockClickSubmitter{
		submitFn: func(event analytics.ClickEvent) {
			submitted <- event
		},
	}

	req := buildRedirectRequest("abc1234")
	req.Header.Set("User-Agent", "GoTestClient/1.0")

	h := handler.NewRedirectHandler(svc, dispatcher)
	w := httptest.NewRecorder()
	h.Handle(w, req)

	require.Equal(t, http.StatusMovedPermanently, w.Code)

	select {
	case event := <-submitted:
		assert.Equal(t, uint64(42), event.URLID)
		assert.Equal(t, "GoTestClient/1.0", event.UserAgent)
		assert.NotZero(t, event.AccessedAt)
	default:
		t.Fatal("dispatcher.Submit was not called")
	}
}

func TestRedirectHandler_CRLFInjection(t *testing.T) {
	tests := []struct {
		name        string
		rawURL      string
		wantStatus  int
		wantLocPart string
	}{
		{
			name:        "crlf stripped from location",
			rawURL:      "https://example.com/path\r\nfoo",
			wantStatus:  http.StatusMovedPermanently,
			wantLocPart: "https://example.com/pathfoo",
		},
		{
			name:       "null byte stripped, still valid",
			rawURL:     "https://example.com/\x00path",
			wantStatus: http.StatusMovedPermanently,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &mockRedirectorService{
				resolveFn: func(_ context.Context, _ string) (*domain.URL, error) {
					return &domain.URL{
						ID:          1,
						Hash:        "abc1234",
						OriginalURL: tt.rawURL,
					}, nil
				},
			}

			h := handler.NewRedirectHandler(svc, &mockClickSubmitter{})
			w := httptest.NewRecorder()
			req := buildRedirectRequest("abc1234")

			h.Handle(w, req)

			resp := w.Result()
			require.Equal(t, tt.wantStatus, resp.StatusCode)

			if tt.wantStatus == http.StatusMovedPermanently {
				loc := resp.Header.Get("Location")
				assert.False(t, strings.Contains(loc, "\r"), "Location must not contain \\r")
				assert.False(t, strings.Contains(loc, "\n"), "Location must not contain \\n")
				assert.False(t, strings.Contains(loc, "\x00"), "Location must not contain null byte")
				if tt.wantLocPart != "" {
					assert.Equal(t, tt.wantLocPart, loc)
				}
			}
		})
	}
}

func TestRedirectHandler_CachePoisoning_BadScheme(t *testing.T) {
	deleteCalled := false

	svc := &mockRedirectorService{
		resolveFn: func(_ context.Context, _ string) (*domain.URL, error) {
			return &domain.URL{
				ID:          1,
				Hash:        "abc1234",
				OriginalURL: "javascript:alert(1)",
			}, nil
		},
	}
	cache := &mockCacheDeleter{
		deleteFn: func(_ context.Context, _ string) error {
			deleteCalled = true
			return nil
		},
	}

	h := handler.NewRedirectHandlerWithCache(svc, &mockClickSubmitter{}, cache)
	w := httptest.NewRecorder()
	req := buildRedirectRequest("abc1234")

	h.Handle(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	assert.True(t, deleteCalled, "cache.Delete must be called for poisoned entry")
}

type mockCacheDeleter struct {
	deleteFn func(ctx context.Context, hash string) error
}

func (m *mockCacheDeleter) Delete(ctx context.Context, hash string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, hash)
	}
	return nil
}

func TestRedirectHandler_NoSubmitOnError(t *testing.T) {
	submitCalled := false

	svc := &mockRedirectorService{
		resolveFn: func(_ context.Context, _ string) (*domain.URL, error) {
			return nil, domain.ErrURLNotFound
		},
	}
	dispatcher := &mockClickSubmitter{
		submitFn: func(_ analytics.ClickEvent) {
			submitCalled = true
		},
	}

	h := handler.NewRedirectHandler(svc, dispatcher)
	w := httptest.NewRecorder()
	h.Handle(w, buildRedirectRequest("zzzzzzz"))

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.False(t, submitCalled, "dispatcher.Submit must not be called on error")
}
