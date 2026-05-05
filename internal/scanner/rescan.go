package scanner

import (
	"context"
	"log/slog"
	"time"

	"github.com/devpedrois/snip/internal/domain"
)

type RescanURLRepository interface {
	FindByVTStatus(ctx context.Context, status string) ([]*domain.URL, error)
	UpdateVTResult(ctx context.Context, id uint64, result ScanResult) error
}

type RescanCacheDeleter interface {
	Delete(ctx context.Context, hash string) error
}

type Rescanner struct {
	scanner URLScanner
	urlRepo RescanURLRepository
	cache   RescanCacheDeleter
	interval time.Duration
}

func NewRescanner(
	scanner URLScanner,
	urlRepo RescanURLRepository,
	cache RescanCacheDeleter,
	interval time.Duration,
) *Rescanner {
	return &Rescanner{
		scanner:  scanner,
		urlRepo:  urlRepo,
		cache:    cache,
		interval: interval,
	}
}

func (r *Rescanner) Run(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.RunCycle(ctx)
		case <-ctx.Done():
			return
		}
	}
}

// RunCycle executes one scan pass over all unverified URLs.
func (r *Rescanner) RunCycle(ctx context.Context) {
	urls, err := r.urlRepo.FindByVTStatus(ctx, string(ScanUnverified))
	if err != nil {
		slog.Error("rescanner: find unverified urls failed", "err", err)
		return
	}

	if len(urls) == 0 {
		return
	}

	slog.Info("rescanner: starting cycle", "count", len(urls))

	for _, u := range urls {
		r.processOne(ctx, u)
		if ctx.Err() != nil {
			return
		}
	}
}

func (r *Rescanner) processOne(ctx context.Context, u *domain.URL) {
	result, _ := r.scanner.Scan(ctx, u.OriginalURL)

	if err := r.urlRepo.UpdateVTResult(ctx, u.ID, result); err != nil {
		slog.Error("rescanner: update vt result failed", "hash", u.Hash, "err", err)
	}

	if result.Status == ScanMalicious {
		if err := r.cache.Delete(ctx, u.Hash); err != nil {
			slog.Warn("rescanner: delete cache key failed", "hash", u.Hash, "err", err)
		}
		slog.Warn("rescanner: url flagged malicious",
			"hash", u.Hash,
			"positives", result.Positives,
			"total", result.Total,
			"report", result.Permalink,
		)
	}

	select {
	case <-time.After(20 * time.Second):
	case <-ctx.Done():
		return
	}
}
