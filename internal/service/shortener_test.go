package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devpedrois/snip/internal/domain"
	"github.com/devpedrois/snip/internal/scanner"
	"github.com/devpedrois/snip/internal/service"
)

const testBaseURL = "http://localhost:8080"

var testVTTimeout = 5 * time.Second

type mockURLRepository struct {
	createFn            func(ctx context.Context, u *domain.URL) error
	updateHashFn        func(ctx context.Context, id uint64, h string) error
	updateVTResultFn    func(ctx context.Context, id uint64, result scanner.ScanResult) error
	findByOriginalURLFn func(ctx context.Context, originalURL string) (*domain.URL, error)
}

func (m *mockURLRepository) Create(ctx context.Context, u *domain.URL) error {
	return m.createFn(ctx, u)
}

func (m *mockURLRepository) UpdateHash(ctx context.Context, id uint64, h string) error {
	return m.updateHashFn(ctx, id, h)
}

func (m *mockURLRepository) UpdateVTResult(ctx context.Context, id uint64, result scanner.ScanResult) error {
	if m.updateVTResultFn != nil {
		return m.updateVTResultFn(ctx, id, result)
	}
	return nil
}

func (m *mockURLRepository) FindByOriginalURL(ctx context.Context, originalURL string) (*domain.URL, error) {
	if m.findByOriginalURLFn != nil {
		return m.findByOriginalURLFn(ctx, originalURL)
	}
	return nil, domain.ErrURLNotFound
}

type mockScanner struct {
	scanFn func(ctx context.Context, url string) (scanner.ScanResult, error)
}

func (m *mockScanner) Scan(ctx context.Context, url string) (scanner.ScanResult, error) {
	if m.scanFn != nil {
		return m.scanFn(ctx, url)
	}
	return scanner.ScanResult{Status: scanner.ScanClean, ScannedAt: time.Now()}, nil
}

func newSvc(repo service.URLRepository, sc scanner.URLScanner) service.ShortenerService {
	return service.NewShortenerService(repo, nil, sc, testBaseURL, 30, testVTTimeout)
}

func TestShortenerService_Shorten(t *testing.T) {
	tests := []struct {
		name           string
		longURL        string
		createFn       func(ctx context.Context, u *domain.URL) error
		updateHashFn   func(ctx context.Context, id uint64, h string) error
		wantErr        error
		wantErrContain string
		wantHashLen    int
	}{
		{
			name:    "success",
			longURL: "https://example.com/path?q=1",
			createFn: func(_ context.Context, u *domain.URL) error {
				u.ID = 1
				return nil
			},
			updateHashFn: func(_ context.Context, _ uint64, _ string) error {
				return nil
			},
			wantHashLen: 7,
		},
		{
			name:    "invalid url - no scheme",
			longURL: "not-a-url",
			createFn: func(_ context.Context, _ *domain.URL) error {
				return nil
			},
			updateHashFn: func(_ context.Context, _ uint64, _ string) error {
				return nil
			},
			wantErr: domain.ErrInvalidURL,
		},
		{
			name:    "invalid url - empty",
			longURL: "",
			createFn: func(_ context.Context, _ *domain.URL) error {
				return nil
			},
			updateHashFn: func(_ context.Context, _ uint64, _ string) error {
				return nil
			},
			wantErr: domain.ErrInvalidURL,
		},
		{
			name:    "repo create error",
			longURL: "https://example.com",
			createFn: func(_ context.Context, _ *domain.URL) error {
				return errors.New("db error")
			},
			updateHashFn: func(_ context.Context, _ uint64, _ string) error {
				return nil
			},
			wantErr:        errors.New("db error"),
			wantErrContain: "db error",
		},
		{
			name:    "repo update hash error",
			longURL: "https://example.com",
			createFn: func(_ context.Context, u *domain.URL) error {
				u.ID = 42
				return nil
			},
			updateHashFn: func(_ context.Context, _ uint64, _ string) error {
				return errors.New("update error")
			},
			wantErr:        errors.New("update error"),
			wantErrContain: "update error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockURLRepository{
				createFn:     tt.createFn,
				updateHashFn: tt.updateHashFn,
			}
			svc := newSvc(repo, &mockScanner{})

			u, _, err := svc.Shorten(context.Background(), tt.longURL)

			if tt.wantErr != nil {
				require.Error(t, err)
				if errors.Is(tt.wantErr, domain.ErrInvalidURL) {
					assert.ErrorIs(t, err, domain.ErrInvalidURL)
				}
				if tt.wantErrContain != "" {
					assert.ErrorContains(t, err, tt.wantErrContain)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, u)
			assert.Len(t, u.Hash, tt.wantHashLen)
			assert.NotNil(t, u.ExpiresAt)
		})
	}
}

func TestShortenerService_Deduplication(t *testing.T) {
	existingURL := &domain.URL{
		ID:          5,
		Hash:        "abc1234",
		OriginalURL: "https://example.com",
	}

	createCalled := false
	repo := &mockURLRepository{
		findByOriginalURLFn: func(_ context.Context, _ string) (*domain.URL, error) {
			return existingURL, nil
		},
		createFn: func(_ context.Context, _ *domain.URL) error {
			createCalled = true
			return nil
		},
		updateHashFn: func(_ context.Context, _ uint64, _ string) error { return nil },
	}

	svc := newSvc(repo, &mockScanner{})

	u1, _, err := svc.Shorten(context.Background(), "https://example.com")
	require.NoError(t, err)
	require.NotNil(t, u1)

	u2, _, err := svc.Shorten(context.Background(), "https://example.com")
	require.NoError(t, err)
	require.NotNil(t, u2)

	assert.Equal(t, u1.Hash, u2.Hash, "same URL must return same hash")
	assert.False(t, createCalled, "Create must not be called for duplicate URL")
}

func TestShortenerService_PopulatesCache(t *testing.T) {
	setCalled := false
	repo := &mockURLRepository{
		createFn: func(_ context.Context, u *domain.URL) error {
			u.ID = 1
			return nil
		},
		updateHashFn: func(_ context.Context, _ uint64, _ string) error { return nil },
	}
	cache := &mockURLCache{
		getFn: func(_ context.Context, _ string) (string, string, uint64, error) {
			return "", "", 0, domain.ErrURLNotFound
		},
		setFn: func(_ context.Context, hash string, originalURL string, vtStatus string, urlID uint64, ttl time.Duration) error {
			setCalled = true
			assert.NotEmpty(t, hash)
			assert.Equal(t, uint64(1), urlID)
			return nil
		},
		deleteFn: func(_ context.Context, _ string) error { return nil },
	}

	svc := service.NewShortenerService(repo, cache, &mockScanner{}, testBaseURL, 30, testVTTimeout)
	u, _, err := svc.Shorten(context.Background(), "https://example.com")

	require.NoError(t, err)
	require.NotNil(t, u)
	assert.True(t, setCalled, "cache.Set should be called after shorten")
}

func TestShortenerService_CacheSetFailure_DoesNotFailShorten(t *testing.T) {
	repo := &mockURLRepository{
		createFn: func(_ context.Context, u *domain.URL) error {
			u.ID = 1
			return nil
		},
		updateHashFn: func(_ context.Context, _ uint64, _ string) error { return nil },
	}
	cache := &mockURLCache{
		getFn: func(_ context.Context, _ string) (string, string, uint64, error) {
			return "", "", 0, domain.ErrURLNotFound
		},
		setFn: func(_ context.Context, _ string, _ string, _ string, _ uint64, _ time.Duration) error {
			return errors.New("redis down")
		},
		deleteFn: func(_ context.Context, _ string) error { return nil },
	}

	svc := service.NewShortenerService(repo, cache, &mockScanner{}, testBaseURL, 30, testVTTimeout)
	u, _, err := svc.Shorten(context.Background(), "https://example.com")

	require.NoError(t, err, "shorten should succeed even if cache.Set fails")
	require.NotNil(t, u)
}

func TestShortenerService_ScanMalicious_RejectsURL(t *testing.T) {
	createCalled := false
	repo := &mockURLRepository{
		createFn: func(_ context.Context, _ *domain.URL) error {
			createCalled = true
			return nil
		},
		updateHashFn: func(_ context.Context, _ uint64, _ string) error { return nil },
	}
	sc := &mockScanner{
		scanFn: func(_ context.Context, _ string) (scanner.ScanResult, error) {
			return scanner.ScanResult{
				Status:    scanner.ScanMalicious,
				Positives: 5,
				Total:     90,
				Permalink: "https://virustotal.com/...",
			}, nil
		},
	}

	svc := newSvc(repo, sc)
	_, _, err := svc.Shorten(context.Background(), "https://example.com")

	require.ErrorIs(t, err, domain.ErrURLMalicious)
	assert.False(t, createCalled, "Create must not be called for malicious URLs")
}

func TestShortenerService_ScanUnverified_SavesURL(t *testing.T) {
	var savedVTStatus string
	repo := &mockURLRepository{
		createFn: func(_ context.Context, u *domain.URL) error {
			u.ID = 1
			return nil
		},
		updateHashFn: func(_ context.Context, _ uint64, _ string) error { return nil },
		updateVTResultFn: func(_ context.Context, _ uint64, result scanner.ScanResult) error {
			savedVTStatus = string(result.Status)
			return nil
		},
	}
	sc := &mockScanner{
		scanFn: func(_ context.Context, _ string) (scanner.ScanResult, error) {
			return scanner.ScanResult{Status: scanner.ScanUnverified}, nil
		},
	}

	svc := newSvc(repo, sc)
	u, result, err := svc.Shorten(context.Background(), "https://example.com")

	require.NoError(t, err)
	require.NotNil(t, u)
	assert.Equal(t, scanner.ScanUnverified, result.Status)
	assert.Equal(t, "unverified", savedVTStatus)
}

func TestShortenerService_ScanAlwaysUnverified_StillWorks(t *testing.T) {
	repo := &mockURLRepository{
		createFn: func(_ context.Context, u *domain.URL) error {
			u.ID = 1
			return nil
		},
		updateHashFn: func(_ context.Context, _ uint64, _ string) error { return nil },
	}
	sc := &mockScanner{
		scanFn: func(_ context.Context, _ string) (scanner.ScanResult, error) {
			return scanner.ScanResult{Status: scanner.ScanUnverified}, nil
		},
	}

	svc := newSvc(repo, sc)
	u, _, err := svc.Shorten(context.Background(), "https://example.com")

	require.NoError(t, err)
	require.NotNil(t, u)
	assert.Len(t, u.Hash, 7)
}
