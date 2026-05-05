package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devpedrois/snip/internal/domain"
	"github.com/devpedrois/snip/internal/handler"
	"github.com/devpedrois/snip/internal/middleware"
	"github.com/devpedrois/snip/internal/scanner"
)

type mockShortenerService struct {
	shortenFn func(ctx context.Context, longURL string) (*domain.URL, scanner.ScanResult, error)
}

func (m *mockShortenerService) Shorten(ctx context.Context, longURL string) (*domain.URL, scanner.ScanResult, error) {
	return m.shortenFn(ctx, longURL)
}

func TestShortenHandler_Handle(t *testing.T) {
	tests := []struct {
		name       string
		body       any
		shortenFn  func(ctx context.Context, longURL string) (*domain.URL, scanner.ScanResult, error)
		wantStatus int
		wantCode   string
		wantHash   string
		wantHeader string
	}{
		{
			name: "201 created",
			body: map[string]string{"url": "https://example.com/path"},
			shortenFn: func(_ context.Context, _ string) (*domain.URL, scanner.ScanResult, error) {
				return &domain.URL{Hash: "abc1234", OriginalURL: "https://example.com/path"},
					scanner.ScanResult{Status: scanner.ScanClean}, nil
			},
			wantStatus: http.StatusCreated,
			wantHash:   "abc1234",
		},
		{
			name: "201 with unverified header",
			body: map[string]string{"url": "https://example.com/path"},
			shortenFn: func(_ context.Context, _ string) (*domain.URL, scanner.ScanResult, error) {
				return &domain.URL{Hash: "abc1234", OriginalURL: "https://example.com/path"},
					scanner.ScanResult{Status: scanner.ScanUnverified}, nil
			},
			wantStatus: http.StatusCreated,
			wantHash:   "abc1234",
			wantHeader: "unverified",
		},
		{
			name: "422 malicious url",
			body: map[string]string{"url": "https://evil.com"},
			shortenFn: func(_ context.Context, _ string) (*domain.URL, scanner.ScanResult, error) {
				return nil, scanner.ScanResult{
					Status:    scanner.ScanMalicious,
					Positives: 5,
					Total:     90,
					Permalink: "https://virustotal.com/report",
				}, domain.ErrURLMalicious
			},
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "ERR_URL_MALICIOUS",
		},
		{
			name: "400 invalid url",
			body: map[string]string{"url": "not-a-url"},
			shortenFn: func(_ context.Context, _ string) (*domain.URL, scanner.ScanResult, error) {
				return nil, scanner.ScanResult{}, domain.ErrInvalidURL
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "ERR_INVALID_URL",
		},
		{
			name:       "400 invalid body",
			body:       "not json",
			shortenFn:  nil,
			wantStatus: http.StatusBadRequest,
			wantCode:   "ERR_INVALID_BODY",
		},
		{
			name: "500 internal error",
			body: map[string]string{"url": "https://example.com"},
			shortenFn: func(_ context.Context, _ string) (*domain.URL, scanner.ScanResult, error) {
				return nil, scanner.ScanResult{}, errors.New("db down")
			},
			wantStatus: http.StatusInternalServerError,
			wantCode:   "ERR_INTERNAL",
		},
		{
			name:       "400 unknown field",
			body:       map[string]string{"url": "https://example.com", "extra": "nope"},
			shortenFn:  nil,
			wantStatus: http.StatusBadRequest,
			wantCode:   "ERR_UNKNOWN_FIELDS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &mockShortenerService{}
			if tt.shortenFn != nil {
				svc.shortenFn = tt.shortenFn
			}

			h := handler.NewShortenHandler(svc, "http://localhost:8080")

			bodyBytes, err := json.Marshal(tt.body)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			h.Handle(w, req)

			res := w.Result()
			assert.Equal(t, tt.wantStatus, res.StatusCode)

			if tt.wantHash != "" {
				var resp handler.ShortenResponse
				require.NoError(t, json.NewDecoder(res.Body).Decode(&resp))
				assert.Equal(t, tt.wantHash, resp.Hash)
				assert.Equal(t, "http://localhost:8080/abc1234", resp.ShortURL)
			}

			if tt.wantCode != "" {
				if tt.wantStatus == http.StatusUnprocessableEntity {
					var errResp handler.MaliciousErrorResponse
					require.NoError(t, json.NewDecoder(res.Body).Decode(&errResp))
					assert.Equal(t, tt.wantCode, errResp.Code)
					assert.Contains(t, errResp.Details, "5/90")
					assert.Equal(t, "https://virustotal.com/report", errResp.Report)
				} else {
					var errResp handler.ErrorResponse
					require.NoError(t, json.NewDecoder(res.Body).Decode(&errResp))
					assert.Equal(t, tt.wantCode, errResp.Code)
				}
			}

			if tt.wantHeader != "" {
				assert.Equal(t, tt.wantHeader, res.Header.Get("X-Scan-Status"))
			}
		})
	}
}

func TestShortenHandler_BodyTooLarge(t *testing.T) {
	svc := &mockShortenerService{}
	h := handler.NewShortenHandler(svc, "http://localhost:8080")

	largeBody := strings.Repeat("a", (1<<20)+1)
	payload := `{"url":"` + largeBody + `"}`

	req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	middleware.BodyLimit(1<<20)(http.HandlerFunc(h.Handle)).ServeHTTP(w, req)

	res := w.Result()
	assert.Equal(t, http.StatusRequestEntityTooLarge, res.StatusCode)

	var errResp handler.ErrorResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&errResp))
	assert.Equal(t, "ERR_BODY_TOO_LARGE", errResp.Code)
}
