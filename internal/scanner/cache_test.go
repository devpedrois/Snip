package scanner_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	redisrepo "github.com/devpedrois/snip/internal/repository/redis"
	"github.com/devpedrois/snip/internal/scanner"
)


type mockScanner struct {
	scanFn func(ctx context.Context, url string) (scanner.ScanResult, error)
}

func (m *mockScanner) Scan(ctx context.Context, url string) (scanner.ScanResult, error) {
	if m.scanFn != nil {
		return m.scanFn(ctx, url)
	}
	return scanner.ScanResult{Status: scanner.ScanClean}, nil
}

func newTestCachedScanner(t *testing.T, inner scanner.URLScanner) (*scanner.CachedScanner, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	cache := redisrepo.NewRedisURLCache(client)
	return scanner.NewCachedScanner(inner, cache, 24*time.Hour), mr
}

func TestCachedScanner_Hit(t *testing.T) {
	callCount := 0
	inner := &mockScanner{
		scanFn: func(_ context.Context, _ string) (scanner.ScanResult, error) {
			callCount++
			return scanner.ScanResult{Status: scanner.ScanClean}, nil
		},
	}

	cs, _ := newTestCachedScanner(t, inner)
	ctx := context.Background()

	r1, err := cs.Scan(ctx, "https://example.com")
	require.NoError(t, err)
	assert.Equal(t, scanner.ScanClean, r1.Status)
	assert.Equal(t, 1, callCount)

	r2, err := cs.Scan(ctx, "https://example.com")
	require.NoError(t, err)
	assert.Equal(t, scanner.ScanClean, r2.Status)
	assert.Equal(t, 1, callCount, "inner scanner must not be called on cache hit")
}

func TestCachedScanner_Miss(t *testing.T) {
	callCount := 0
	inner := &mockScanner{
		scanFn: func(_ context.Context, _ string) (scanner.ScanResult, error) {
			callCount++
			return scanner.ScanResult{Status: scanner.ScanClean}, nil
		},
	}

	cs, _ := newTestCachedScanner(t, inner)
	ctx := context.Background()

	r1, err := cs.Scan(ctx, "https://example.com")
	require.NoError(t, err)
	assert.Equal(t, scanner.ScanClean, r1.Status)

	r2, err := cs.Scan(ctx, "https://different.com")
	require.NoError(t, err)
	assert.Equal(t, scanner.ScanClean, r2.Status)
	assert.Equal(t, 2, callCount, "different URLs should each call inner scanner")
}

func TestCachedScanner_MaliciousCached(t *testing.T) {
	callCount := 0
	inner := &mockScanner{
		scanFn: func(_ context.Context, _ string) (scanner.ScanResult, error) {
			callCount++
			return scanner.ScanResult{Status: scanner.ScanMalicious, Positives: 5, Total: 90}, nil
		},
	}

	cs, _ := newTestCachedScanner(t, inner)
	ctx := context.Background()

	r1, err := cs.Scan(ctx, "https://evil.com")
	require.NoError(t, err)
	assert.Equal(t, scanner.ScanMalicious, r1.Status)
	assert.Equal(t, 1, callCount)

	r2, err := cs.Scan(ctx, "https://evil.com")
	require.NoError(t, err)
	assert.Equal(t, scanner.ScanMalicious, r2.Status)
	assert.Equal(t, 1, callCount, "malicious result must be served from cache without re-scan")
}

func TestCachedScanner_RedisError_FallsBackToInner(t *testing.T) {
	callCount := 0
	inner := &mockScanner{
		scanFn: func(_ context.Context, _ string) (scanner.ScanResult, error) {
			callCount++
			return scanner.ScanResult{Status: scanner.ScanClean}, nil
		},
	}

	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	cache := redisrepo.NewRedisURLCache(client)
	cs := scanner.NewCachedScanner(inner, cache, 24*time.Hour)

	mr.Close()

	result, err := cs.Scan(context.Background(), "https://example.com")

	require.NoError(t, err)
	assert.Equal(t, scanner.ScanClean, result.Status)
	assert.Equal(t, 1, callCount, "should fall back to inner scanner on Redis error")
}
