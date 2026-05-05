package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/devpedrois/snip/internal/domain"
)

const (
	urlKeyPrefix  = "snip:url:"
	metaKeyPrefix = "snip:meta:"
)

type urlCacheEntry struct {
	OriginalURL string `json:"original_url"`
	VTStatus    string `json:"vt_status"`
}

type URLCache interface {
	Get(ctx context.Context, hash string) (originalURL string, vtStatus string, urlID uint64, err error)
	Set(ctx context.Context, hash string, originalURL string, vtStatus string, urlID uint64, ttl time.Duration) error
	Delete(ctx context.Context, hash string) error
}

type RedisURLCache struct {
	client *goredis.Client
}

func NewRedisURLCache(client *goredis.Client) *RedisURLCache {
	return &RedisURLCache{client: client}
}

func (c *RedisURLCache) Get(ctx context.Context, hash string) (string, string, uint64, error) {
	results, err := c.client.MGet(ctx, urlKeyPrefix+hash, metaKeyPrefix+hash).Result()
	if err != nil {
		return "", "", 0, fmt.Errorf("cache: get %q: %w", hash, err)
	}

	urlVal, ok := results[0].(string)
	if !ok || urlVal == "" {
		return "", "", 0, domain.ErrURLNotFound
	}

	var originalURL, vtStatus string
	var entry urlCacheEntry
	if json.Unmarshal([]byte(urlVal), &entry) == nil {
		originalURL = entry.OriginalURL
		vtStatus = entry.VTStatus
	} else {
		originalURL = urlVal
	}

	var urlID uint64
	if idStr, ok := results[1].(string); ok && idStr != "" {
		if parsed, parseErr := strconv.ParseUint(idStr, 10, 64); parseErr == nil {
			urlID = parsed
		}
	}

	return originalURL, vtStatus, urlID, nil
}

func (c *RedisURLCache) Set(ctx context.Context, hash string, originalURL string, vtStatus string, urlID uint64, ttl time.Duration) error {
	entry := urlCacheEntry{OriginalURL: originalURL, VTStatus: vtStatus}
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("cache: marshal entry %q: %w", hash, err)
	}

	pipe := c.client.Pipeline()
	pipe.Set(ctx, urlKeyPrefix+hash, string(data), ttl)
	pipe.Set(ctx, metaKeyPrefix+hash, strconv.FormatUint(urlID, 10), ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("cache: set %q: %w", hash, err)
	}
	return nil
}

func (c *RedisURLCache) Delete(ctx context.Context, hash string) error {
	if err := c.client.Del(ctx, urlKeyPrefix+hash, metaKeyPrefix+hash).Err(); err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil
		}
		return fmt.Errorf("cache: delete %q: %w", hash, err)
	}
	return nil
}

func (c *RedisURLCache) GetRaw(ctx context.Context, key string) ([]byte, error) {
	data, err := c.client.Get(ctx, key).Bytes()
	if errors.Is(err, goredis.Nil) {
		return nil, fmt.Errorf("cache: raw get %q: not found", key)
	}
	if err != nil {
		return nil, fmt.Errorf("cache: raw get %q: %w", key, err)
	}
	return data, nil
}

func (c *RedisURLCache) SetRaw(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := c.client.Set(ctx, key, value, ttl).Err(); err != nil {
		return fmt.Errorf("cache: raw set %q: %w", key, err)
	}
	return nil
}
