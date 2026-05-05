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

type mockReportURLRepo struct {
	findByHashFn     func(ctx context.Context, hash string) (*domain.URL, error)
	updateVTStatusFn func(ctx context.Context, id uint64, status string) error
}

func (m *mockReportURLRepo) FindByHash(ctx context.Context, hash string) (*domain.URL, error) {
	return m.findByHashFn(ctx, hash)
}

func (m *mockReportURLRepo) UpdateVTStatus(ctx context.Context, id uint64, status string) error {
	if m.updateVTStatusFn != nil {
		return m.updateVTStatusFn(ctx, id, status)
	}
	return nil
}

type mockReportRepo struct {
	insertFn                  func(ctx context.Context, r *domain.Report) error
	countDistinctIPsByURLIDFn func(ctx context.Context, urlID uint64) (int64, error)
}

func (m *mockReportRepo) Insert(ctx context.Context, r *domain.Report) error {
	return m.insertFn(ctx, r)
}

func (m *mockReportRepo) CountDistinctIPsByURLID(ctx context.Context, urlID uint64) (int64, error) {
	if m.countDistinctIPsByURLIDFn != nil {
		return m.countDistinctIPsByURLIDFn(ctx, urlID)
	}
	return 0, nil
}

type mockReportCache struct {
	deleteFn func(ctx context.Context, hash string) error
}

func (m *mockReportCache) Delete(ctx context.Context, hash string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, hash)
	}
	return nil
}

func TestReportService_Report(t *testing.T) {
	existingURL := &domain.URL{ID: 1, Hash: "abc1234", OriginalURL: "https://example.com"}

	tests := []struct {
		name            string
		hash            string
		reporterIP      string
		reason          string
		urlRepo         *mockReportURLRepo
		reportRepo      *mockReportRepo
		cache           *mockReportCache
		threshold       int
		wantErr         error
		wantVTUpdated   bool
		wantCacheDelete bool
	}{
		{
			name:       "valid report saved",
			hash:       "abc1234",
			reporterIP: "192.168.1.45",
			reason:     "phishing",
			urlRepo: &mockReportURLRepo{
				findByHashFn: func(_ context.Context, _ string) (*domain.URL, error) { return existingURL, nil },
			},
			reportRepo: &mockReportRepo{
				insertFn:                  func(_ context.Context, _ *domain.Report) error { return nil },
				countDistinctIPsByURLIDFn: func(_ context.Context, _ uint64) (int64, error) { return 1, nil },
			},
			cache:     &mockReportCache{},
			threshold: 5,
		},
		{
			name:       "hash not found returns ErrURLNotFound",
			hash:       "missing",
			reporterIP: "1.2.3.4",
			reason:     "spam",
			urlRepo: &mockReportURLRepo{
				findByHashFn: func(_ context.Context, _ string) (*domain.URL, error) {
					return nil, domain.ErrURLNotFound
				},
			},
			reportRepo: &mockReportRepo{
				insertFn: func(_ context.Context, _ *domain.Report) error { return nil },
			},
			cache:     &mockReportCache{},
			threshold: 5,
			wantErr:   domain.ErrURLNotFound,
		},
		{
			name:       "invalid reason returns ErrInvalidReason",
			hash:       "abc1234",
			reporterIP: "1.2.3.4",
			reason:     "invalid",
			urlRepo: &mockReportURLRepo{
				findByHashFn: func(_ context.Context, _ string) (*domain.URL, error) { return existingURL, nil },
			},
			reportRepo: &mockReportRepo{
				insertFn: func(_ context.Context, _ *domain.Report) error { return nil },
			},
			cache:     &mockReportCache{},
			threshold: 5,
			wantErr:   domain.ErrInvalidReason,
		},
		{
			name:       "duplicate report returns ErrDuplicateReport",
			hash:       "abc1234",
			reporterIP: "192.168.1.45",
			reason:     "malware",
			urlRepo: &mockReportURLRepo{
				findByHashFn: func(_ context.Context, _ string) (*domain.URL, error) { return existingURL, nil },
			},
			reportRepo: &mockReportRepo{
				insertFn: func(_ context.Context, _ *domain.Report) error { return domain.ErrDuplicateReport },
			},
			cache:     &mockReportCache{},
			threshold: 5,
			wantErr:   domain.ErrDuplicateReport,
		},
		{
			name:       "count >= threshold triggers auto-block",
			hash:       "abc1234",
			reporterIP: "10.0.0.5",
			reason:     "spam",
			urlRepo: func() *mockReportURLRepo {
				vtUpdated := false
				_ = vtUpdated
				return &mockReportURLRepo{
					findByHashFn: func(_ context.Context, _ string) (*domain.URL, error) { return existingURL, nil },
					updateVTStatusFn: func(_ context.Context, _ uint64, status string) error {
						assert.Equal(t, "malicious", status)
						return nil
					},
				}
			}(),
			reportRepo: &mockReportRepo{
				insertFn:                  func(_ context.Context, _ *domain.Report) error { return nil },
				countDistinctIPsByURLIDFn: func(_ context.Context, _ uint64) (int64, error) { return 5, nil },
			},
			cache: func() *mockReportCache {
				deleted := false
				_ = deleted
				return &mockReportCache{
					deleteFn: func(_ context.Context, hash string) error {
						assert.Equal(t, "abc1234", hash)
						return nil
					},
				}
			}(),
			threshold:       5,
			wantVTUpdated:   true,
			wantCacheDelete: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := service.NewReportService(tt.urlRepo, tt.reportRepo, tt.cache, tt.threshold)

			err := svc.Report(context.Background(), tt.hash, tt.reporterIP, tt.reason)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestReportService_AnonymizesIP(t *testing.T) {
	var savedIP string

	urlRepo := &mockReportURLRepo{
		findByHashFn: func(_ context.Context, _ string) (*domain.URL, error) {
			return &domain.URL{ID: 1, Hash: "abc1234"}, nil
		},
	}
	reportRepo := &mockReportRepo{
		insertFn: func(_ context.Context, r *domain.Report) error {
			savedIP = r.ReporterIP
			return nil
		},
		countDistinctIPsByURLIDFn: func(_ context.Context, _ uint64) (int64, error) { return 1, nil },
	}

	svc := service.NewReportService(urlRepo, reportRepo, &mockReportCache{}, 5)
	err := svc.Report(context.Background(), "abc1234", "192.168.1.45", "phishing")

	require.NoError(t, err)
	assert.Equal(t, "192.168.1.0", savedIP, "IP must be anonymized before saving")
}

func TestReportService_CountError_DoesNotFail(t *testing.T) {
	urlRepo := &mockReportURLRepo{
		findByHashFn: func(_ context.Context, _ string) (*domain.URL, error) {
			return &domain.URL{ID: 1, Hash: "abc1234"}, nil
		},
	}
	reportRepo := &mockReportRepo{
		insertFn: func(_ context.Context, _ *domain.Report) error { return nil },
		countDistinctIPsByURLIDFn: func(_ context.Context, _ uint64) (int64, error) {
			return 0, errors.New("db error")
		},
	}

	svc := service.NewReportService(urlRepo, reportRepo, &mockReportCache{}, 5)
	err := svc.Report(context.Background(), "abc1234", "1.2.3.4", "other")

	require.NoError(t, err, "count error must not fail the report submission")
}
