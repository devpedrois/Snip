package redis_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devpedrois/snip/internal/domain"
	redisrepo "github.com/devpedrois/snip/internal/repository/redis"
)

func newTestCache(t *testing.T) (*redisrepo.RedisURLCache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	return redisrepo.NewRedisURLCache(client), mr
}

func TestRedisURLCache_SetAndGet(t *testing.T) {
	cache, _ := newTestCache(t)
	ctx := context.Background()

	err := cache.Set(ctx, "abc1234", "https://example.com", "clean", 7, 30*24*time.Hour)
	require.NoError(t, err)

	gotURL, gotVTStatus, gotID, err := cache.Get(ctx, "abc1234")
	require.NoError(t, err)
	assert.Equal(t, "https://example.com", gotURL)
	assert.Equal(t, "clean", gotVTStatus)
	assert.Equal(t, uint64(7), gotID)
}

func TestRedisURLCache_SetAndGet_VTStatusPreserved(t *testing.T) {
	cache, _ := newTestCache(t)
	ctx := context.Background()

	require.NoError(t, cache.Set(ctx, "unv1234", "https://pending.com", "unverified", 3, time.Hour))

	gotURL, gotStatus, gotID, err := cache.Get(ctx, "unv1234")
	require.NoError(t, err)
	assert.Equal(t, "https://pending.com", gotURL)
	assert.Equal(t, "unverified", gotStatus)
	assert.Equal(t, uint64(3), gotID)
}

func TestRedisURLCache_Get_Miss(t *testing.T) {
	cache, _ := newTestCache(t)

	_, _, _, err := cache.Get(context.Background(), "notexist")
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrURLNotFound))
}

func TestRedisURLCache_Delete(t *testing.T) {
	cache, _ := newTestCache(t)
	ctx := context.Background()

	require.NoError(t, cache.Set(ctx, "abc1234", "https://example.com", "clean", 1, time.Hour))
	require.NoError(t, cache.Delete(ctx, "abc1234"))

	_, _, _, err := cache.Get(ctx, "abc1234")
	assert.True(t, errors.Is(err, domain.ErrURLNotFound))
}

func TestRedisURLCache_TTL(t *testing.T) {
	cache, mr := newTestCache(t)
	ctx := context.Background()

	require.NoError(t, cache.Set(ctx, "abc1234", "https://example.com", "clean", 1, time.Second))

	mr.FastForward(2 * time.Second)

	_, _, _, err := cache.Get(ctx, "abc1234")
	assert.True(t, errors.Is(err, domain.ErrURLNotFound))
}

func TestRedisURLCache_Get_BackwardCompatPlainString(t *testing.T) {
	cache, mr := newTestCache(t)
	ctx := context.Background()

	require.NoError(t, mr.Set("snip:url:legacyH", "https://legacy.com"))

	gotURL, gotStatus, _, err := cache.Get(ctx, "legacyH")
	require.NoError(t, err)
	assert.Equal(t, "https://legacy.com", gotURL)
	assert.Empty(t, gotStatus, "backward-compat entry should have empty vt_status")
}
