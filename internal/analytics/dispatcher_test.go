package analytics_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devpedrois/snip/internal/analytics"
	"github.com/devpedrois/snip/internal/domain"
)

type mockClickInserter struct {
	insertFn func(ctx context.Context, c *domain.Click) error
}

func (m *mockClickInserter) Insert(ctx context.Context, c *domain.Click) error {
	return m.insertFn(ctx, c)
}

type mockLastAccessUpdater struct {
	updateFn func(ctx context.Context, id uint64) error
}

func (m *mockLastAccessUpdater) UpdateLastAccessed(ctx context.Context, id uint64) error {
	return m.updateFn(ctx, id)
}

func noopClickRepo() *mockClickInserter {
	return &mockClickInserter{insertFn: func(_ context.Context, _ *domain.Click) error { return nil }}
}

func noopURLRepo() *mockLastAccessUpdater {
	return &mockLastAccessUpdater{updateFn: func(_ context.Context, _ uint64) error { return nil }}
}

func TestDispatcher_Submit_addsEventToChannel(t *testing.T) {
	d := analytics.NewDispatcher(noopClickRepo(), noopURLRepo(), 1, 10)

	d.Submit(analytics.ClickEvent{URLID: 1, AccessedAt: time.Now()})

	assert.Equal(t, 1, d.QueueLen())
	assert.Equal(t, int64(0), d.Dropped())
}

func TestDispatcher_Submit_dropsWhenBufferFull(t *testing.T) {
	const bufferSize = 3
	d := analytics.NewDispatcher(noopClickRepo(), noopURLRepo(), 0, bufferSize)

	for i := 0; i < bufferSize; i++ {
		d.Submit(analytics.ClickEvent{URLID: uint64(i + 1), AccessedAt: time.Now()})
	}

	d.Submit(analytics.ClickEvent{URLID: 99, AccessedAt: time.Now()})

	assert.Equal(t, int64(1), d.Dropped())
	assert.Equal(t, bufferSize, d.QueueLen())
}

func TestDispatcher_worker_processesEvents(t *testing.T) {
	insertCalled := make(chan *domain.Click, 1)
	updateCalled := make(chan uint64, 1)

	clickRepo := &mockClickInserter{
		insertFn: func(_ context.Context, c *domain.Click) error {
			insertCalled <- c
			return nil
		},
	}
	urlRepo := &mockLastAccessUpdater{
		updateFn: func(_ context.Context, id uint64) error {
			updateCalled <- id
			return nil
		},
	}

	d := analytics.NewDispatcher(clickRepo, urlRepo, 1, 10)
	d.Run(context.Background())

	event := analytics.ClickEvent{
		URLID:      42,
		AccessedAt: time.Now().UTC().Truncate(time.Second),
		UserAgent:  "test-agent",
		IP:         "192.168.1.1",
	}
	d.Submit(event)

	select {
	case click := <-insertCalled:
		assert.Equal(t, uint64(42), click.URLID)
		assert.Equal(t, "test-agent", click.UserAgent)
		assert.Equal(t, "192.168.1.1", click.IP)
	case <-time.After(time.Second):
		t.Fatal("clickRepo.Insert not called within timeout")
	}

	select {
	case id := <-updateCalled:
		assert.Equal(t, uint64(42), id)
	case <-time.After(time.Second):
		t.Fatal("urlRepo.UpdateLastAccessed not called within timeout")
	}

	d.Shutdown(time.Second)
	assert.Equal(t, int64(1), d.Processed())
}

func TestDispatcher_worker_continuesAfterRepoError(t *testing.T) {
	var processed int64

	clickRepo := &mockClickInserter{
		insertFn: func(_ context.Context, c *domain.Click) error {
			if c.URLID == 1 {
				return assert.AnError
			}
			atomic.AddInt64(&processed, 1)
			return nil
		},
	}

	d := analytics.NewDispatcher(clickRepo, noopURLRepo(), 1, 10)
	d.Run(context.Background())

	d.Submit(analytics.ClickEvent{URLID: 1})
	d.Submit(analytics.ClickEvent{URLID: 2})

	d.Shutdown(time.Second)

	assert.Equal(t, int64(1), atomic.LoadInt64(&processed), "second event must still process after first fails")
}

func TestDispatcher_Shutdown_drainsRemainingEvents(t *testing.T) {
	var processed int64

	clickRepo := &mockClickInserter{
		insertFn: func(_ context.Context, _ *domain.Click) error {
			atomic.AddInt64(&processed, 1)
			return nil
		},
	}

	d := analytics.NewDispatcher(clickRepo, noopURLRepo(), 2, 20)
	d.Run(context.Background())

	const total = 10
	for i := 0; i < total; i++ {
		d.Submit(analytics.ClickEvent{URLID: uint64(i + 1), AccessedAt: time.Now()})
	}

	d.Shutdown(2 * time.Second)

	require.Equal(t, int64(total), atomic.LoadInt64(&processed), "all events must be processed before shutdown completes")
}
