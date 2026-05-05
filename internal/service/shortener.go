package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/devpedrois/snip/internal/domain"
	"github.com/devpedrois/snip/internal/hash"
	"github.com/devpedrois/snip/internal/scanner"
)

type URLRepository interface {
	Create(ctx context.Context, u *domain.URL) error
	UpdateHash(ctx context.Context, id uint64, h string) error
	UpdateVTResult(ctx context.Context, id uint64, result scanner.ScanResult) error
	FindByOriginalURL(ctx context.Context, originalURL string) (*domain.URL, error)
}

type ShortenerService interface {
	Shorten(ctx context.Context, longURL string) (*domain.URL, scanner.ScanResult, error)
}

type shortenerService struct {
	repo              URLRepository
	cache             URLCache
	scan              scanner.URLScanner
	baseURL           string
	urlExpirationDays int
	vtTimeout         time.Duration
}

func NewShortenerService(
	repo URLRepository,
	cache URLCache,
	scan scanner.URLScanner,
	baseURL string,
	urlExpirationDays int,
	vtTimeout time.Duration,
) *shortenerService {
	return &shortenerService{
		repo:              repo,
		cache:             cache,
		scan:              scan,
		baseURL:           baseURL,
		urlExpirationDays: urlExpirationDays,
		vtTimeout:         vtTimeout,
	}
}

func (s *shortenerService) Shorten(ctx context.Context, longURL string) (*domain.URL, scanner.ScanResult, error) {
	normalized, err := scanner.NormalizeURL(longURL)
	if err != nil {
		return nil, scanner.ScanResult{}, fmt.Errorf("shortener: normalize: %w", domain.ErrInvalidURL)
	}

	if err := hash.ValidateURL(normalized, s.baseURL); err != nil {
		return nil, scanner.ScanResult{}, err
	}

	existing, err := s.repo.FindByOriginalURL(ctx, normalized)
	if err != nil && !errors.Is(err, domain.ErrURLNotFound) {
		return nil, scanner.ScanResult{}, fmt.Errorf("shortener: dedup check: %w", err)
	}
	if existing != nil {
		return existing, scanner.ScanResult{Status: scanner.ScanStatus(existing.VTStatus)}, nil
	}

	scanCtx, cancel := context.WithTimeout(ctx, s.vtTimeout)
	defer cancel()

	result, _ := s.scan.Scan(scanCtx, normalized)

	if result.Status == scanner.ScanMalicious {
		return nil, result, domain.ErrURLMalicious
	}

	expiresAt := time.Now().AddDate(0, 0, s.urlExpirationDays)
	u := &domain.URL{
		OriginalURL: normalized,
		ExpiresAt:   &expiresAt,
	}

	if err := s.repo.Create(ctx, u); err != nil {
		return nil, scanner.ScanResult{}, fmt.Errorf("shortener: create: %w", err)
	}

	h := hash.Encode(u.ID)

	if err := s.repo.UpdateHash(ctx, u.ID, h); err != nil {
		return nil, scanner.ScanResult{}, fmt.Errorf("shortener: update hash: %w", err)
	}

	u.Hash = h

	if err := s.repo.UpdateVTResult(ctx, u.ID, result); err != nil {
		slog.Warn("shortener: update vt result failed", "hash", h, "err", err)
	}

	u.VTStatus = string(result.Status)

	if s.cache != nil {
		ttl := time.Duration(s.urlExpirationDays) * 24 * time.Hour
		if err := s.cache.Set(ctx, h, normalized, u.VTStatus, u.ID, ttl); err != nil {
			slog.Warn("shortener: cache set failed", "hash", h, "err", err)
		}
	}

	return u, result, nil
}
