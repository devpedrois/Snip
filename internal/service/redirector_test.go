package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devpedrois/snip/internal/domain"
	"github.com/devpedrois/snip/internal/service"
)

type mockRedirectURLRepo struct {
	findByHashFn         func(ctx context.Context, hash string) (*domain.URL, error)
	updateLastAccessedFn func(ctx context.Context, id uint64) error
}

func (m *mockRedirectURLRepo) FindByHash(ctx context.Context, hash string) (*domain.URL, error) {
	return m.findByHashFn(ctx, hash)
}

func (m *mockRedirectURLRepo) UpdateLastAccessed(ctx context.Context, id uint64) error {
	return m.updateLastAccessedFn(ctx, id)
}

type mockURLCache struct {
	getFn    func(ctx context.Context, hash string) (string, string, uint64, error)
	setFn    func(ctx context.Context, hash string, originalURL string, vtStatus string, urlID uint64, ttl time.Duration) error
	deleteFn func(ctx context.Context, hash string) error
}

func (m *mockURLCache) Get(ctx context.Context, hash string) (string, string, uint64, error) {
	return m.getFn(ctx, hash)
}

func (m *mockURLCache) Set(ctx context.Context, hash string, originalURL string, vtStatus string, urlID uint64, ttl time.Duration) error {
	return m.setFn(ctx, hash, originalURL, vtStatus, urlID, ttl)
}

func (m *mockURLCache) Delete(ctx context.Context, hash string) error {
	return m.deleteFn(ctx, hash)
}

func TestRedirectorService_Resolve(t *testing.T) {
	pastTime := time.Now().Add(-24 * time.Hour)
	futureTime := time.Now().Add(30 * 24 * time.Hour)

	tests := []struct {
		name       string
		hash       string
		findResult *domain.URL
		findErr    error
		expectErr  error
		expectURL  string
		expectID   uint64
	}{
		{
			name: "hit: returns original URL and ID",
			hash: "abc1234",
			findResult: &domain.URL{
				ID:          1,
				Hash:        "abc1234",
				OriginalURL: "https://example.com",
				ExpiresAt:   &futureTime,
				VTStatus:    "clean",
			},
			expectURL: "https://example.com",
			expectID:  1,
		},
		{
			name:      "miss: not found returns ErrURLNotFound",
			hash:      "zzzzzzz",
			findErr:   domain.ErrURLNotFound,
			expectErr: domain.ErrURLNotFound,
		},
		{
			name: "expired: returns ErrURLExpired",
			hash: "expired",
			findResult: &domain.URL{
				ID:          2,
				Hash:        "expired",
				OriginalURL: "https://old.com",
				ExpiresAt:   &pastTime,
			},
			expectErr: domain.ErrURLExpired,
		},
		{
			name: "no expiry: never expires, returns URL",
			hash: "noexp1",
			findResult: &domain.URL{
				ID:          3,
				Hash:        "noexp1",
				OriginalURL: "https://noexpiry.com",
				ExpiresAt:   nil,
			},
			expectURL: "https://noexpiry.com",
			expectID:  3,
		},
		{
			name: "invalid scheme in DB: returns ErrInvalidURL",
			hash: "bad1234",
			findResult: &domain.URL{
				ID:          4,
				Hash:        "bad1234",
				OriginalURL: "javascript:alert(1)",
				ExpiresAt:   &futureTime,
			},
			expectErr: domain.ErrInvalidURL,
		},
		{
			name: "malicious status: returns ErrURLMalicious",
			hash: "mal1234",
			findResult: &domain.URL{
				ID:          5,
				Hash:        "mal1234",
				OriginalURL: "https://evil.com",
				ExpiresAt:   &futureTime,
				VTStatus:    "malicious",
			},
			expectErr: domain.ErrURLMalicious,
		},
		{
			name: "unverified status: redirect proceeds normally",
			hash: "unv1234",
			findResult: &domain.URL{
				ID:          6,
				Hash:        "unv1234",
				OriginalURL: "https://pending.com",
				ExpiresAt:   &futureTime,
				VTStatus:    "unverified",
			},
			expectURL: "https://pending.com",
			expectID:  6,
		},
		{
			name: "clean status: redirect proceeds normally",
			hash: "cln1234",
			findResult: &domain.URL{
				ID:          7,
				Hash:        "cln1234",
				OriginalURL: "https://safe.com",
				ExpiresAt:   &futureTime,
				VTStatus:    "clean",
			},
			expectURL: "https://safe.com",
			expectID:  7,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &mockRedirectURLRepo{
				findByHashFn: func(_ context.Context, hash string) (*domain.URL, error) {
					return tc.findResult, tc.findErr
				},
				updateLastAccessedFn: func(_ context.Context, id uint64) error { return nil },
			}

			svc := service.NewRedirectorService(repo, nil, 30)
			u, err := svc.Resolve(context.Background(), tc.hash)

			if tc.expectErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tc.expectErr))
				return
			}

			require.NoError(t, err)
			require.NotNil(t, u)
			assert.Equal(t, tc.expectURL, u.OriginalURL)
			assert.Equal(t, tc.expectID, u.ID)
		})
	}
}

func TestRedirectorService_CacheHit(t *testing.T) {
	repoCalled := false
	repo := &mockRedirectURLRepo{
		findByHashFn: func(_ context.Context, _ string) (*domain.URL, error) {
			repoCalled = true
			return nil, errors.New("should not be called")
		},
		updateLastAccessedFn: func(_ context.Context, _ uint64) error { return nil },
	}

	cache := &mockURLCache{
		getFn: func(_ context.Context, hash string) (string, string, uint64, error) {
			return "https://cached.com", "clean", 5, nil
		},
		setFn:    func(_ context.Context, _ string, _ string, _ string, _ uint64, _ time.Duration) error { return nil },
		deleteFn: func(_ context.Context, _ string) error { return nil },
	}

	svc := service.NewRedirectorService(repo, cache, 30)
	u, err := svc.Resolve(context.Background(), "abc1234")

	require.NoError(t, err)
	require.NotNil(t, u)
	assert.Equal(t, "https://cached.com", u.OriginalURL)
	assert.Equal(t, uint64(5), u.ID)
	assert.False(t, repoCalled, "repo must not be called on cache hit")
}

func TestRedirectorService_CacheMiss_PopulatesCache(t *testing.T) {
	futureTime := time.Now().Add(30 * 24 * time.Hour)
	setCalled := false

	repo := &mockRedirectURLRepo{
		findByHashFn: func(_ context.Context, _ string) (*domain.URL, error) {
			return &domain.URL{
				ID:          1,
				Hash:        "abc1234",
				OriginalURL: "https://example.com",
				ExpiresAt:   &futureTime,
				VTStatus:    "clean",
			}, nil
		},
		updateLastAccessedFn: func(_ context.Context, _ uint64) error { return nil },
	}

	cache := &mockURLCache{
		getFn: func(_ context.Context, _ string) (string, string, uint64, error) {
			return "", "", 0, domain.ErrURLNotFound
		},
		setFn: func(_ context.Context, hash string, originalURL string, vtStatus string, urlID uint64, ttl time.Duration) error {
			setCalled = true
			assert.Equal(t, "abc1234", hash)
			assert.Equal(t, "https://example.com", originalURL)
			assert.Equal(t, "clean", vtStatus)
			assert.Equal(t, uint64(1), urlID)
			return nil
		},
		deleteFn: func(_ context.Context, _ string) error { return nil },
	}

	svc := service.NewRedirectorService(repo, cache, 30)
	u, err := svc.Resolve(context.Background(), "abc1234")

	require.NoError(t, err)
	require.NotNil(t, u)
	assert.Equal(t, "https://example.com", u.OriginalURL)
	assert.Equal(t, uint64(1), u.ID)
	assert.True(t, setCalled, "cache.Set must be called after DB lookup")
}

func TestRedirectorService_CacheInfraError_FallsBackToRepo(t *testing.T) {
	futureTime := time.Now().Add(30 * 24 * time.Hour)
	repoCalled := false

	repo := &mockRedirectURLRepo{
		findByHashFn: func(_ context.Context, _ string) (*domain.URL, error) {
			repoCalled = true
			return &domain.URL{
				ID:          1,
				Hash:        "abc1234",
				OriginalURL: "https://example.com",
				ExpiresAt:   &futureTime,
				VTStatus:    "clean",
			}, nil
		},
		updateLastAccessedFn: func(_ context.Context, _ uint64) error { return nil },
	}

	cache := &mockURLCache{
		getFn: func(_ context.Context, _ string) (string, string, uint64, error) {
			return "", "", 0, errors.New("redis: connection refused")
		},
		setFn:    func(_ context.Context, _ string, _ string, _ string, _ uint64, _ time.Duration) error { return nil },
		deleteFn: func(_ context.Context, _ string) error { return nil },
	}

	svc := service.NewRedirectorService(repo, cache, 30)
	u, err := svc.Resolve(context.Background(), "abc1234")

	require.NoError(t, err)
	require.NotNil(t, u)
	assert.Equal(t, "https://example.com", u.OriginalURL)
	assert.Equal(t, uint64(1), u.ID)
	assert.True(t, repoCalled, "repo must be called as fallback on cache infra error")
}

func TestRedirectorService_MaliciousFromDB_DeletesCache(t *testing.T) {
	futureTime := time.Now().Add(30 * 24 * time.Hour)
	permalink := "https://virustotal.com/gui/url/abc"
	deleteCalled := false

	repo := &mockRedirectURLRepo{
		findByHashFn: func(_ context.Context, _ string) (*domain.URL, error) {
			return &domain.URL{
				ID:          1,
				Hash:        "mal1234",
				OriginalURL: "https://evil.com",
				ExpiresAt:   &futureTime,
				VTStatus:    "malicious",
				VTPermalink: &permalink,
			}, nil
		},
		updateLastAccessedFn: func(_ context.Context, _ uint64) error { return nil },
	}

	cache := &mockURLCache{
		getFn: func(_ context.Context, _ string) (string, string, uint64, error) {
			return "", "", 0, domain.ErrURLNotFound
		},
		setFn: func(_ context.Context, _ string, _ string, _ string, _ uint64, _ time.Duration) error {
			return nil
		},
		deleteFn: func(_ context.Context, _ string) error {
			deleteCalled = true
			return nil
		},
	}

	svc := service.NewRedirectorService(repo, cache, 30)
	u, err := svc.Resolve(context.Background(), "mal1234")

	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrURLMalicious))
	require.NotNil(t, u)
	assert.Equal(t, "malicious", u.VTStatus)
	assert.True(t, deleteCalled, "cache.Delete must be called for malicious URL")
}

func TestRedirectorService_MaliciousFromCache_DeletesAndBlocks(t *testing.T) {
	repoCalled := false
	deleteCalled := false

	repo := &mockRedirectURLRepo{
		findByHashFn: func(_ context.Context, _ string) (*domain.URL, error) {
			repoCalled = true
			return nil, errors.New("should not be called")
		},
		updateLastAccessedFn: func(_ context.Context, _ uint64) error { return nil },
	}

	cache := &mockURLCache{
		getFn: func(_ context.Context, hash string) (string, string, uint64, error) {
			return "https://evil.com", "malicious", 1, nil
		},
		setFn: func(_ context.Context, _ string, _ string, _ string, _ uint64, _ time.Duration) error {
			return nil
		},
		deleteFn: func(_ context.Context, _ string) error {
			deleteCalled = true
			return nil
		},
	}

	svc := service.NewRedirectorService(repo, cache, 30)
	u, err := svc.Resolve(context.Background(), "mal1234")

	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrURLMalicious))
	require.NotNil(t, u)
	assert.False(t, repoCalled, "repo must not be called when cache hit is malicious")
	assert.True(t, deleteCalled, "cache.Delete must be called to evict stale malicious key")
}
