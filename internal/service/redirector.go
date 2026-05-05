package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/devpedrois/snip/internal/domain"
	"github.com/devpedrois/snip/internal/scanner"
)

type URLReader interface {
	FindByHash(ctx context.Context, hash string) (*domain.URL, error)
	UpdateLastAccessed(ctx context.Context, id uint64) error
}

type URLCache interface {
	Get(ctx context.Context, hash string) (originalURL string, vtStatus string, urlID uint64, err error)
	Set(ctx context.Context, hash string, originalURL string, vtStatus string, urlID uint64, ttl time.Duration) error
	Delete(ctx context.Context, hash string) error
}

type RedirectorService interface {
	Resolve(ctx context.Context, hash string) (*domain.URL, error)
}

type redirectorService struct {
	repo     URLReader
	cache    URLCache
	cacheTTL time.Duration
}

func NewRedirectorService(repo URLReader, cache URLCache, cacheTTLDays int) *redirectorService {
	return &redirectorService{
		repo:     repo,
		cache:    cache,
		cacheTTL: time.Duration(cacheTTLDays) * 24 * time.Hour,
	}
}

func (s *redirectorService) Resolve(ctx context.Context, hash string) (*domain.URL, error) {
	if s.cache != nil {
		originalURL, vtStatus, urlID, err := s.cache.Get(ctx, hash)
		if err == nil {
			if vtStatus == string(scanner.ScanMalicious) {
				if delErr := s.cache.Delete(ctx, hash); delErr != nil {
					slog.Warn("redirector: delete malicious cache key failed", "hash", hash, "err", delErr)
				}
				return &domain.URL{Hash: hash, VTStatus: vtStatus}, domain.ErrURLMalicious
			}
			return &domain.URL{
				Hash:        hash,
				OriginalURL: originalURL,
				ID:          urlID,
				VTStatus:    vtStatus,
			}, nil
		}
		if !errors.Is(err, domain.ErrURLNotFound) {
			slog.Warn("redirector: cache get failed, falling back to db", "hash", hash, "err", err)
		}
	}

	u, err := s.repo.FindByHash(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("redirector: find: %w", err)
	}

	if u.ExpiresAt != nil && u.ExpiresAt.Before(time.Now()) {
		return nil, domain.ErrURLExpired
	}

	parsed, parseErr := url.Parse(u.OriginalURL)
	if parseErr != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, domain.ErrInvalidURL
	}

	if u.VTStatus == string(scanner.ScanMalicious) {
		if s.cache != nil {
			if delErr := s.cache.Delete(ctx, hash); delErr != nil {
				slog.Warn("redirector: delete malicious cache key failed", "hash", hash, "err", delErr)
			}
		}
		return u, domain.ErrURLMalicious
	}

	if s.cache != nil {
		if err := s.cache.Set(ctx, hash, u.OriginalURL, u.VTStatus, u.ID, s.cacheTTL); err != nil {
			slog.Warn("redirector: cache set failed", "hash", hash, "err", err)
		}
	}

	return u, nil
}
