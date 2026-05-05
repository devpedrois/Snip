package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devpedrois/snip/internal/domain"
	"github.com/devpedrois/snip/internal/handler"
	"github.com/devpedrois/snip/internal/service"
)

type mockAnalyticsService struct {
	stats *service.Stats
	err   error
}

func (m *mockAnalyticsService) GetStats(_ context.Context, _ string) (*service.Stats, error) {
	return m.stats, m.err
}

func buildAnalyticsRequest(hash string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/api/analytics/"+hash, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("hash", hash)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestAnalyticsHandler_Handle(t *testing.T) {
	tests := []struct {
		name           string
		hash           string
		svcStats       *service.Stats
		svcErr         error
		expectedStatus int
		checkBody      func(t *testing.T, body []byte)
	}{
		{
			name: "200 com stats JSON",
			hash: "abc1234",
			svcStats: &service.Stats{
				TotalClicks: 42,
				ClicksByDay: []domain.DailyCount{
					{Date: "2026-04-29", Count: 15},
				},
				TopUserAgents: []domain.UserAgentCount{
					{UserAgent: "curl/8.5.0", Count: 42},
				},
			},
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body []byte) {
				var got service.Stats
				require.NoError(t, json.Unmarshal(body, &got))
				assert.Equal(t, int64(42), got.TotalClicks)
				assert.Len(t, got.ClicksByDay, 1)
				assert.Equal(t, "2026-04-29", got.ClicksByDay[0].Date)
				assert.Len(t, got.TopUserAgents, 1)
				assert.Equal(t, "curl/8.5.0", got.TopUserAgents[0].UserAgent)
			},
		},
		{
			name:           "404 hash inexistente",
			hash:           "naoexis",
			svcErr:         domain.ErrURLNotFound,
			expectedStatus: http.StatusNotFound,
			checkBody: func(t *testing.T, body []byte) {
				var got handler.ErrorResponse
				require.NoError(t, json.Unmarshal(body, &got))
				assert.Equal(t, "ERR_NOT_FOUND", got.Code)
			},
		},
		{
			name:           "500 erro interno",
			hash:           "abc1234",
			svcErr:         errors.New("db connection failed"),
			expectedStatus: http.StatusInternalServerError,
			checkBody: func(t *testing.T, body []byte) {
				var got handler.ErrorResponse
				require.NoError(t, json.Unmarshal(body, &got))
				assert.Equal(t, "ERR_INTERNAL", got.Code)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &mockAnalyticsService{stats: tc.svcStats, err: tc.svcErr}
			h := handler.NewAnalyticsHandler(svc)

			w := httptest.NewRecorder()
			req := buildAnalyticsRequest(tc.hash)
			h.Handle(w, req)

			resp := w.Result()
			require.Equal(t, tc.expectedStatus, resp.StatusCode)

			if tc.checkBody != nil {
				tc.checkBody(t, w.Body.Bytes())
			}
		})
	}
}
