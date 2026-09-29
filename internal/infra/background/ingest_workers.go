package background

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/ingest_track"
)

// Lane is a share of the queue with workers of its own, so a Provider that
// has to download slowly holds up nobody else.
type Lane struct {
	Filter  repositories.JobFilter
	Workers int
}

type IngestWorkers struct {
	ProcessIngestJob *ingest_track.ProcessIngestJob
	InFlight         *ingest_track.InFlight
	Queue            repositories.IngestQueue
	SettleBatches    *ingest_track.SettleIngestBatches
	Disk             common.Disk
	Waker            *Waker

	Lanes []Lane
	// PollInterval: queued jobs wake the workers, so polling finds only
	// the jobs whose retry has come.
	PollInterval time.Duration
	// Processed hears of each job the workers have taken on.
	Processed func()
}

// Start runs the workers until ctx is done; the returned channel closes
// once all of them have stopped.
func (w *IngestWorkers) Start(ctx context.Context) <-chan struct{} {
	if err := w.Disk.ClearScratch(); err != nil {
		slog.Error("clear_ingest_scratch", "error", err)
	}

	if err := w.SettleBatches.Execute(ctx); err != nil {
		slog.Error("settle_ingest_batches", "error", err)
	}

	var wg sync.WaitGroup
	for _, lane := range w.Lanes {
		wake := w.Waker.lane()
		for range lane.Workers {
			wg.Go(func() { w.loop(ctx, lane.Filter, wake) })
		}
	}

	stopped := make(chan struct{})
	go func() {
		wg.Wait()
		close(stopped)
	}()
	return stopped
}

// WaitIdle blocks until no job is pending and no worker is still
// finishing one.
func (w *IngestWorkers) WaitIdle(ctx context.Context) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		unfinished, err := w.Queue.CountUnfinished(ctx)
		if err != nil {
			return err
		}
		if unfinished == 0 && w.InFlight.Idle() {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (w *IngestWorkers) loop(ctx context.Context, filter repositories.JobFilter, wake <-chan struct{}) {
	ticker := time.NewTicker(w.PollInterval)
	defer ticker.Stop()

	for {
		for w.ProcessIngestJob.Execute(ctx, filter) {
			w.Processed()
		}

		select {
		case <-ctx.Done():
			return
		case <-wake:
		case <-ticker.C:
		}
	}
}
