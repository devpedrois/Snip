package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devpedrois/snip/internal/domain"
	"github.com/devpedrois/snip/internal/handler"
	"github.com/devpedrois/snip/internal/middleware"
)

type mockReportService struct {
	reportFn func(ctx context.Context, hash, reporterIP, reason string) error
}

func (m *mockReportService) Report(ctx context.Context, hash, reporterIP, reason string) error {
	return m.reportFn(ctx, hash, reporterIP, reason)
}

func reportRequest(t *testing.T, hash string, body any, contentType string) (*httptest.ResponseRecorder, *http.Request) {
	t.Helper()

	var bodyBytes []byte
	var err error
	if s, ok := body.(string); ok {
		bodyBytes = []byte(s)
	} else {
		bodyBytes, err = json.Marshal(body)
		require.NoError(t, err)
	}

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("hash", hash)

	req := httptest.NewRequest(http.MethodPost, "/api/report/"+hash, bytes.NewReader(bodyBytes))
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.RemoteAddr = "192.168.1.45:12345"

	return httptest.NewRecorder(), req
}

func TestReportHandler_Handle(t *testing.T) {
	tests := []struct {
		name        string
		hash        string
		body        any
		contentType string
		reportFn    func(ctx context.Context, hash, reporterIP, reason string) error
		wantStatus  int
		wantCode    string
		wantMessage string
	}{
		{
			name:        "201 report received",
			hash:        "abc1234",
			body:        map[string]string{"reason": "phishing"},
			contentType: "application/json",
			reportFn:    func(_ context.Context, _, _, _ string) error { return nil },
			wantStatus:  http.StatusCreated,
			wantMessage: "report received",
		},
		{
			name:        "404 hash not found",
			hash:        "missing1",
			body:        map[string]string{"reason": "spam"},
			contentType: "application/json",
			reportFn:    func(_ context.Context, _, _, _ string) error { return domain.ErrURLNotFound },
			wantStatus:  http.StatusNotFound,
			wantCode:    "ERR_NOT_FOUND",
		},
		{
			name:        "400 invalid reason",
			hash:        "abc1234",
			body:        map[string]string{"reason": "badvalue"},
			contentType: "application/json",
			reportFn:    func(_ context.Context, _, _, _ string) error { return domain.ErrInvalidReason },
			wantStatus:  http.StatusBadRequest,
			wantCode:    "ERR_INVALID_REASON",
		},
		{
			name:        "409 duplicate report",
			hash:        "abc1234",
			body:        map[string]string{"reason": "malware"},
			contentType: "application/json",
			reportFn:    func(_ context.Context, _, _, _ string) error { return domain.ErrDuplicateReport },
			wantStatus:  http.StatusConflict,
			wantCode:    "ERR_DUPLICATE_REPORT",
		},
		{
			name:        "400 unknown fields in body",
			hash:        "abc1234",
			body:        map[string]string{"reason": "spam", "extra": "nope"},
			contentType: "application/json",
			reportFn:    func(_ context.Context, _, _, _ string) error { return nil },
			wantStatus:  http.StatusBadRequest,
			wantCode:    "ERR_UNKNOWN_FIELDS",
		},
		{
			name:        "400 invalid json body",
			hash:        "abc1234",
			body:        "not json",
			contentType: "application/json",
			reportFn:    func(_ context.Context, _, _, _ string) error { return nil },
			wantStatus:  http.StatusBadRequest,
			wantCode:    "ERR_INVALID_BODY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &mockReportService{reportFn: tt.reportFn}
			h := handler.NewReportHandler(svc)

			w, req := reportRequest(t, tt.hash, tt.body, tt.contentType)
			h.Handle(w, req)

			res := w.Result()
			assert.Equal(t, tt.wantStatus, res.StatusCode)

			if tt.wantMessage != "" {
				var resp handler.ReportResponse
				require.NoError(t, json.NewDecoder(res.Body).Decode(&resp))
				assert.Equal(t, tt.wantMessage, resp.Message)
			}

			if tt.wantCode != "" {
				var errResp handler.ErrorResponse
				require.NoError(t, json.NewDecoder(res.Body).Decode(&errResp))
				assert.Equal(t, tt.wantCode, errResp.Code)
			}
		})
	}
}

func TestReportHandler_NoContentType_Returns415(t *testing.T) {
	svc := &mockReportService{reportFn: func(_ context.Context, _, _, _ string) error { return nil }}
	h := handler.NewReportHandler(svc)

	body := `{"reason":"spam"}`
	req := httptest.NewRequest(http.MethodPost, "/api/report/abc1234", strings.NewReader(body))
	req.RemoteAddr = "1.2.3.4:5678"
	w := httptest.NewRecorder()

	middleware.RequireJSON(http.HandlerFunc(h.Handle)).ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnsupportedMediaType, w.Result().StatusCode)
}
