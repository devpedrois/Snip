package scanner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"time"
)

type RawCache interface {
	GetRaw(ctx context.Context, key string) ([]byte, error)
	SetRaw(ctx context.Context, key string, value []byte, ttl time.Duration) error
}

type CachedScanner struct {
	inner RawCache
	scan  URLScanner
	ttl   time.Duration
}

func NewCachedScanner(inner URLScanner, cache RawCache, ttl time.Duration) *CachedScanner {
	return &CachedScanner{scan: inner, inner: cache, ttl: ttl}
}

func (c *CachedScanner) Scan(ctx context.Context, rawURL string) (ScanResult, error) {
	key := cacheKey(rawURL)

	if data, err := c.inner.GetRaw(ctx, key); err == nil {
		var result ScanResult
		if json.Unmarshal(data, &result) == nil {
			return result, nil
		}
	}

	result, err := c.scan.Scan(ctx, rawURL)

	data, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		slog.Warn("scanner: cache: marshal result", "err", marshalErr)
		return result, err
	}

	if setErr := c.inner.SetRaw(ctx, key, data, c.ttl); setErr != nil {
		slog.Warn("scanner: cache: set failed", "err", setErr)
	}

	return result, err
}

func cacheKey(rawURL string) string {
	sum := sha256.Sum256([]byte(rawURL))
	return "snip:vt:" + hex.EncodeToString(sum[:])
}
