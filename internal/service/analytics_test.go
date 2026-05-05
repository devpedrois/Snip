package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devpedrois/snip/internal/domain"
	"github.com/devpedrois/snip/internal/service"
)

type mockURLFinder struct {
	url *domain.URL
	err error
}

func (m *mockURLFinder) FindByHash(_ context.Context, _ string) (*domain.URL, error) {
	return m.url, m.err
}

type mockClickReader struct {
	count        int64
	countErr     error
	dailyCounts  []domain.DailyCount
	groupErr     error
	userAgents   []domain.UserAgentCount
	userAgentErr error
}

func (m *mockClickReader) CountByURLID(_ context.Context, _ uint64) (int64, error) {
	return m.count, m.countErr
}

func (m *mockClickReader) GroupByDay(_ context.Context, _ uint64, _ int) ([]domain.DailyCount, error) {
	return m.dailyCounts, m.groupErr
}

func (m *mockClickReader) TopUserAgents(_ context.Context, _ uint64, _ int) ([]domain.UserAgentCount, error) {
	return m.userAgents, m.userAgentErr
}

func TestAnalyticsService_GetStats(t *testing.T) {
	tests := []struct {
		name        string
		hash        string
		urlFinder   *mockURLFinder
		clickReader *mockClickReader
		wantErr     error
		wantStats   *service.Stats
	}{
		{
			name: "hash valido com clicks",
			hash: "abc1234",
			urlFinder: &mockURLFinder{
				url: &domain.URL{ID: 1, Hash: "abc1234"},
			},
			clickReader: &mockClickReader{
				count: 42,
				dailyCounts: []domain.DailyCount{
					{Date: "2026-04-29", Count: 15},
					{Date: "2026-04-30", Count: 27},
				},
				userAgents: []domain.UserAgentCount{
					{UserAgent: "curl/8.5.0", Count: 42},
				},
			},
			wantStats: &service.Stats{
				TotalClicks: 42,
				ClicksByDay: []domain.DailyCount{
					{Date: "2026-04-29", Count: 15},
					{Date: "2026-04-30", Count: 27},
				},
				TopUserAgents: []domain.UserAgentCount{
					{UserAgent: "curl/8.5.0", Count: 42},
				},
			},
		},
		{
			name: "hash valido sem clicks",
			hash: "abc1234",
			urlFinder: &mockURLFinder{
				url: &domain.URL{ID: 2, Hash: "abc1234"},
			},
			clickReader: &mockClickReader{
				count:       0,
				dailyCounts: nil,
				userAgents:  nil,
			},
			wantStats: &service.Stats{
				TotalClicks:   0,
				ClicksByDay:   []domain.DailyCount{},
				TopUserAgents: []domain.UserAgentCount{},
			},
		},
		{
			name: "hash inexistente retorna ErrURLNotFound",
			hash: "naoexis",
			urlFinder: &mockURLFinder{
				err: domain.ErrURLNotFound,
			},
			clickReader: &mockClickReader{},
			wantErr:     domain.ErrURLNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := service.NewAnalyticsService(tc.urlFinder, tc.clickReader)
			stats, err := svc.GetStats(context.Background(), tc.hash)

			if tc.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tc.wantErr))
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantStats.TotalClicks, stats.TotalClicks)
			assert.Equal(t, tc.wantStats.ClicksByDay, stats.ClicksByDay)
			assert.Equal(t, tc.wantStats.TopUserAgents, stats.TopUserAgents)
		})
	}
}
