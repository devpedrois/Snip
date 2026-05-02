package service

import (
	"context"
	"fmt"
	"time"

	"github.com/devpedrois/snip/internal/domain"
	"github.com/devpedrois/snip/internal/hash"
)

type URLRepository interface {
	Create(ctx context.Context, u *domain.URL) error
	UpdateHash(ctx context.Context, id uint64, h string) error
}

type ShortenerService interface {
	Shorten(ctx context.Context, longURL string) (*domain.URL, error)
}

type shortenerService struct {
	repo              URLRepository
	baseURL           string
	urlExpirationDays int
}

func NewShortenerService(repo URLRepository, baseURL string, urlExpirationDays int) ShortenerService {
	return &shortenerService{
		repo:              repo,
		baseURL:           baseURL,
		urlExpirationDays: urlExpirationDays,
	}
}

func (s *shortenerService) Shorten(ctx context.Context, longURL string) (*domain.URL, error) {
	if err := hash.ValidateURL(longURL); err != nil {
		return nil, err
	}

	expiresAt := time.Now().AddDate(0, 0, s.urlExpirationDays)
	u := &domain.URL{
		OriginalURL: longURL,
		ExpiresAt:   &expiresAt,
	}

	if err := s.repo.Create(ctx, u); err != nil {
		return nil, fmt.Errorf("shortener: create: %w", err)
	}

	h := hash.Encode(u.ID)

	if err := s.repo.UpdateHash(ctx, u.ID, h); err != nil {
		return nil, fmt.Errorf("shortener: update hash: %w", err)
	}

	u.Hash = h
	return u, nil
}
