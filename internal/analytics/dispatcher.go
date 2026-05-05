package analytics

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/devpedrois/snip/internal/domain"
)

type ClickInserter interface {
	Insert(ctx context.Context, c *domain.Click) error
}

type LastAccessUpdater interface {
	UpdateLastAccessed(ctx context.Context, id uint64) error
}

// ClickCleaner is implemented by repositories that support retention-based deletion.
type ClickCleaner interface {
	DeleteOlderThan(ctx context.Context, days int) (int64, error)
}

type Dispatcher struct {
	events        chan ClickEvent
	clickRepo     ClickInserter
	urlRepo       LastAccessUpdater
	cleaner       ClickCleaner
	retentionDays int
	numWorkers    int
	wg            sync.WaitGroup
	processed     atomic.Int64
	dropped       atomic.Int64
	stopStats     context.CancelFunc
}

func NewDispatcher(clickRepo ClickInserter, urlRepo LastAccessUpdater, numWorkers, bufferSize int) *Dispatcher {
	return &Dispatcher{
		events:     make(chan ClickEvent, bufferSize),
		clickRepo:  clickRepo,
		urlRepo:    urlRepo,
		numWorkers: numWorkers,
	}
}

// WithRetention configures a cleanup goroutine that deletes clicks older than retentionDays.
func (d *Dispatcher) WithRetention(cleaner ClickCleaner, retentionDays int) *Dispatcher {
	d.cleaner = cleaner
	d.retentionDays = retentionDays
	return d
}

func (d *Dispatcher) Submit(event ClickEvent) {
	select {
	case d.events <- event:
	default:
		d.dropped.Add(1)
		slog.Warn("analytics: event dropped, buffer full", "url_id", event.URLID)
	}
}

func (d *Dispatcher) Run(ctx context.Context) {
	statsCtx, cancel := context.WithCancel(ctx)
	d.stopStats = cancel

	for i := 0; i < d.numWorkers; i++ {
		d.wg.Add(1)
		go d.worker()
	}

	go d.statsLoop(statsCtx)

	if d.cleaner != nil {
		go d.cleanupLoop(ctx)
	}
}

func (d *Dispatcher) Shutdown(timeout time.Duration) {
	slog.Info("shutting down dispatcher...")
	close(d.events)

	done := make(chan struct{})
	go func() {
		d.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(timeout):
		slog.Warn("dispatcher shutdown timeout, events may be lost", "remaining", len(d.events))
	}

	if d.stopStats != nil {
		d.stopStats()
	}

	slog.Info("dispatcher shutdown complete", "processed", d.processed.Load(), "dropped", d.dropped.Load())
}

func (d *Dispatcher) QueueLen() int {
	return len(d.events)
}

func (d *Dispatcher) Processed() int64 {
	return d.processed.Load()
}

func (d *Dispatcher) Dropped() int64 {
	return d.dropped.Load()
}

func (d *Dispatcher) worker() {
	defer d.wg.Done()
	for event := range d.events {
		ctx := context.Background()
		click := &domain.Click{
			URLID:      event.URLID,
			AccessedAt: event.AccessedAt,
			UserAgent:  event.UserAgent,
			IP:         event.IP,
		}
		if err := d.clickRepo.Insert(ctx, click); err != nil {
			slog.Error("analytics: insert click failed", "url_id", event.URLID, "err", err)
		}
		if err := d.urlRepo.UpdateLastAccessed(ctx, event.URLID); err != nil {
			slog.Error("analytics: update last accessed failed", "url_id", event.URLID, "err", err)
		}
		d.processed.Add(1)
	}
}

func (d *Dispatcher) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			deleted, err := d.cleaner.DeleteOlderThan(ctx, d.retentionDays)
			if err != nil {
				slog.Error("analytics: click cleanup failed", "err", err)
				continue
			}
			slog.Info("analytics: click cleanup complete", "deleted", deleted, "retention_days", d.retentionDays)
		}
	}
}

func (d *Dispatcher) statsLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			slog.Info("analytics stats",
				"processed", d.processed.Load(),
				"dropped", d.dropped.Load(),
				"queue", len(d.events),
			)
		}
	}
}
