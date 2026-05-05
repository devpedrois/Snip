package scanner_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devpedrois/snip/internal/domain"
	"github.com/devpedrois/snip/internal/scanner"
)

type mockRescanURLRepo struct {
	findByVTStatusFn func(ctx context.Context, status string) ([]*domain.URL, error)
	updateVTResultFn func(ctx context.Context, id uint64, result scanner.ScanResult) error
}

func (m *mockRescanURLRepo) FindByVTStatus(ctx context.Context, status string) ([]*domain.URL, error) {
	return m.findByVTStatusFn(ctx, status)
}

func (m *mockRescanURLRepo) UpdateVTResult(ctx context.Context, id uint64, result scanner.ScanResult) error {
	return m.updateVTResultFn(ctx, id, result)
}

type mockRescanCache struct {
	deleteFn func(ctx context.Context, hash string) error
}

func (m *mockRescanCache) Delete(ctx context.Context, hash string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, hash)
	}
	return nil
}

type mockRescanScanner struct {
	scanFn func(ctx context.Context, url string) (scanner.ScanResult, error)
}

func (m *mockRescanScanner) Scan(ctx context.Context, url string) (scanner.ScanResult, error) {
	return m.scanFn(ctx, url)
}

func TestRescanner_UnverifiedBecomesClean(t *testing.T) {
	updateCalled := false

	repo := &mockRescanURLRepo{
		findByVTStatusFn: func(_ context.Context, status string) ([]*domain.URL, error) {
			assert.Equal(t, "unverified", status)
			return []*domain.URL{
				{ID: 1, Hash: "abc1234", OriginalURL: "https://example.com"},
			}, nil
		},
		updateVTResultFn: func(_ context.Context, id uint64, result scanner.ScanResult) error {
			assert.Equal(t, uint64(1), id)
			assert.Equal(t, scanner.ScanClean, result.Status)
			updateCalled = true
			return nil
		},
	}

	deleteCalled := false
	cache := &mockRescanCache{
		deleteFn: func(_ context.Context, hash string) error {
			deleteCalled = true
			return nil
		},
	}

	sc := &mockRescanScanner{
		scanFn: func(_ context.Context, _ string) (scanner.ScanResult, error) {
			return scanner.ScanResult{Status: scanner.ScanClean}, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())

	r := scanner.NewRescanner(sc, repo, cache, time.Hour)

	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	r.RunCycle(ctx)

	assert.True(t, updateCalled, "vt result must be updated in DB")
	assert.False(t, deleteCalled, "cache must NOT be deleted for clean URL")
}

func TestRescanner_UnverifiedBecomesMalicious_DeletesCache(t *testing.T) {
	updateCalled := false
	deleteCalled := false
	deletedHash := ""

	repo := &mockRescanURLRepo{
		findByVTStatusFn: func(_ context.Context, _ string) ([]*domain.URL, error) {
			return []*domain.URL{
				{ID: 2, Hash: "mal1234", OriginalURL: "https://evil.com"},
			}, nil
		},
		updateVTResultFn: func(_ context.Context, id uint64, result scanner.ScanResult) error {
			assert.Equal(t, uint64(2), id)
			assert.Equal(t, scanner.ScanMalicious, result.Status)
			updateCalled = true
			return nil
		},
	}

	cache := &mockRescanCache{
		deleteFn: func(_ context.Context, hash string) error {
			deleteCalled = true
			deletedHash = hash
			return nil
		},
	}

	sc := &mockRescanScanner{
		scanFn: func(_ context.Context, _ string) (scanner.ScanResult, error) {
			return scanner.ScanResult{
				Status:    scanner.ScanMalicious,
				Positives: 5,
				Total:     72,
				Permalink: "https://virustotal.com/...",
			}, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r := scanner.NewRescanner(sc, repo, cache, time.Hour)
	r.RunCycle(ctx)

	assert.True(t, updateCalled, "vt result must be updated in DB")
	assert.True(t, deleteCalled, "cache key must be deleted when URL becomes malicious")
	assert.Equal(t, "mal1234", deletedHash)
}

func TestRescanner_CtxCancelledDuringSleep_StopsGracefully(t *testing.T) {
	var processedCount int32

	repo := &mockRescanURLRepo{
		findByVTStatusFn: func(_ context.Context, _ string) ([]*domain.URL, error) {
			return []*domain.URL{
				{ID: 1, Hash: "url0001", OriginalURL: "https://a.com"},
				{ID: 2, Hash: "url0002", OriginalURL: "https://b.com"},
				{ID: 3, Hash: "url0003", OriginalURL: "https://c.com"},
			}, nil
		},
		updateVTResultFn: func(_ context.Context, _ uint64, _ scanner.ScanResult) error {
			atomic.AddInt32(&processedCount, 1)
			return nil
		},
	}

	cache := &mockRescanCache{}

	sc := &mockRescanScanner{
		scanFn: func(_ context.Context, _ string) (scanner.ScanResult, error) {
			return scanner.ScanResult{Status: scanner.ScanClean}, nil
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	r := scanner.NewRescanner(sc, repo, cache, time.Hour)

	start := time.Now()
	r.RunCycle(ctx)
	elapsed := time.Since(start)

	assert.Less(t, elapsed, 15*time.Second, "must not wait full 20s sleep when ctx is cancelled")
}

func TestRescanner_EmptyCycle_NoScans(t *testing.T) {
	scanCalled := false

	repo := &mockRescanURLRepo{
		findByVTStatusFn: func(_ context.Context, _ string) ([]*domain.URL, error) {
			return nil, nil
		},
		updateVTResultFn: func(_ context.Context, _ uint64, _ scanner.ScanResult) error {
			return nil
		},
	}

	sc := &mockRescanScanner{
		scanFn: func(_ context.Context, _ string) (scanner.ScanResult, error) {
			scanCalled = true
			return scanner.ScanResult{}, nil
		},
	}

	ctx := context.Background()
	r := scanner.NewRescanner(sc, repo, &mockRescanCache{}, time.Hour)
	r.RunCycle(ctx)

	assert.False(t, scanCalled, "scanner must not be called when no unverified URLs")
}

func TestRescanner_FindError_LogsAndContinues(t *testing.T) {
	repo := &mockRescanURLRepo{
		findByVTStatusFn: func(_ context.Context, _ string) ([]*domain.URL, error) {
			return nil, errors.New("db connection lost")
		},
		updateVTResultFn: func(_ context.Context, _ uint64, _ scanner.ScanResult) error {
			return nil
		},
	}

	sc := &mockRescanScanner{
		scanFn: func(_ context.Context, _ string) (scanner.ScanResult, error) {
			return scanner.ScanResult{}, nil
		},
	}

	r := scanner.NewRescanner(sc, repo, &mockRescanCache{}, time.Hour)
	require.NotPanics(t, func() {
		r.RunCycle(context.Background())
	})
}
