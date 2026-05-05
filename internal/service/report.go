package service

import (
	"context"
	"errors"
	"log/slog"
	"net"

	"github.com/devpedrois/snip/internal/analytics"
	"github.com/devpedrois/snip/internal/domain"
)

// ReportURLRepository is the narrow URL repository interface needed by the report service.
type ReportURLRepository interface {
	FindByHash(ctx context.Context, hash string) (*domain.URL, error)
	UpdateVTStatus(ctx context.Context, id uint64, status string) error
}

// ReportRepository is the persistence interface for abuse reports.
type ReportRepository interface {
	Insert(ctx context.Context, r *domain.Report) error
	CountDistinctIPsByURLID(ctx context.Context, urlID uint64) (int64, error)
}

// ReportCacheDeleter deletes a cached URL entry by hash.
type ReportCacheDeleter interface {
	Delete(ctx context.Context, hash string) error
}

// ReportService handles abuse report submission.
type ReportService interface {
	Report(ctx context.Context, hash, reporterIP, reason string) error
}

type reportService struct {
	urlRepo    ReportURLRepository
	reportRepo ReportRepository
	cache      ReportCacheDeleter
	threshold  int
}

// NewReportService returns a ReportService.
func NewReportService(
	urlRepo ReportURLRepository,
	reportRepo ReportRepository,
	cache ReportCacheDeleter,
	threshold int,
) ReportService {
	return &reportService{
		urlRepo:    urlRepo,
		reportRepo: reportRepo,
		cache:      cache,
		threshold:  threshold,
	}
}

func (s *reportService) Report(ctx context.Context, hash, reporterIP, reason string) error {
	u, err := s.urlRepo.FindByHash(ctx, hash)
	if err != nil {
		return err
	}

	if !domain.IsValidReason(reason) {
		return domain.ErrInvalidReason
	}

	anonIP := analytics.AnonymizeIP(stripPort(reporterIP))

	rep := &domain.Report{
		URLID:      u.ID,
		ReporterIP: anonIP,
		Reason:     reason,
	}

	if err := s.reportRepo.Insert(ctx, rep); err != nil {
		if errors.Is(err, domain.ErrDuplicateReport) {
			return domain.ErrDuplicateReport
		}
		return err
	}

	count, err := s.reportRepo.CountDistinctIPsByURLID(ctx, u.ID)
	if err != nil {
		slog.Warn("report_service: failed to count reports", "url_id", u.ID, "err", err)
		return nil
	}

	if int(count) >= s.threshold {
		if vtErr := s.urlRepo.UpdateVTStatus(ctx, u.ID, "malicious"); vtErr != nil {
			slog.Warn("report_service: failed to update vt status", "url_id", u.ID, "err", vtErr)
		}
		if delErr := s.cache.Delete(ctx, hash); delErr != nil {
			slog.Warn("report_service: failed to delete cache key", "hash", hash, "err", delErr)
		}
		slog.Warn("url auto-blocked due to abuse reports", "hash", hash, "reports", count)
	}

	return nil
}

func stripPort(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}
