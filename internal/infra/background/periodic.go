package background

import (
	"context"
	"log/slog"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/attach_libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/reconcile_libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/resolve_songs"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/sync_collection"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/disk"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/runs"
)

// every runs fn each interval, or sooner when woken, until ctx is done,
// starting with a wait.
func every(ctx context.Context, interval time.Duration, woken <-chan struct{}, fn func(context.Context)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-woken:
		}
		fn(ctx)
	}
}

// Attacher keeps the Attached Libraries in step with Navidrome. A zero
// Interval turns them off.
type Attacher struct {
	Attach   *attach_libraries.AttachLibraries
	Interval time.Duration

	runs runs.Runs
}

// Once is not fatal: Navidrome may be down, and the next run catches up.
func (a *Attacher) Once(ctx context.Context) {
	if a.Interval <= 0 {
		return
	}
	defer a.runs.Start()()
	if err := a.Attach.Execute(ctx); err != nil && ctx.Err() == nil {
		slog.Error("attach_libraries", "error", err)
	}
}

// Run starts with a wait: the bot has attached on its start.
func (a *Attacher) Run(ctx context.Context) {
	if a.Interval <= 0 {
		return
	}
	every(ctx, a.Interval, nil, a.Once)
}

// Wait blocks until a run that starts after the call is over; it lets a
// caller see the effect of a change in Navidrome.
func (a *Attacher) Wait(ctx context.Context) error {
	return a.runs.Wait(ctx)
}

type Reconciler struct {
	Reconcile *reconcile_libraries.ReconcileLibraries
	Interval  time.Duration

	runs runs.Runs
}

func (r *Reconciler) Once(ctx context.Context) {
	defer r.runs.Start()()
	if err := r.Reconcile.Execute(ctx); err != nil && ctx.Err() == nil {
		slog.Error("reconcile_libraries", "error", err)
	}
}

// Run starts with a wait: the bot has reconciled on its start.
func (r *Reconciler) Run(ctx context.Context) {
	every(ctx, r.Interval, nil, r.Once)
}

// Wait blocks until a run that starts after the call is over; it lets a
// caller see the effect of files changed by hand.
func (r *Reconciler) Wait(ctx context.Context) error {
	return r.runs.Wait(ctx)
}

// SongResolver is off with a zero Interval.
type SongResolver struct {
	Resolve  *resolve_songs.ResolveSongs
	Interval time.Duration
}

func (r *SongResolver) Run(ctx context.Context) {
	if r.Interval <= 0 {
		return
	}
	every(ctx, r.Interval, nil, func(ctx context.Context) {
		// Not fatal: Navidrome may be down, and the next run catches up.
		if err := r.Resolve.Execute(ctx); err != nil && ctx.Err() == nil {
			slog.Error("resolve_songs", "error", err)
		}
	})
}

type Scheduler struct {
	Provider provider.ProviderName
	Sync     *sync_collection.SyncCollection
	Tick     time.Duration

	runs runs.Runs
}

func (s *Scheduler) Run(ctx context.Context) {
	s.once(ctx)
	every(ctx, s.Tick, s.runs.Woken(), s.once)
}

// Now runs Sync without waiting for the tick and waits for it to finish.
func (s *Scheduler) Now(ctx context.Context) error {
	return s.runs.Now(ctx)
}

func (s *Scheduler) once(ctx context.Context) {
	defer s.runs.Start()()
	s.Sync.Execute(ctx, s.Provider)
}

// Sweeper removes the scratch files crashed Ingests left behind.
type Sweeper struct {
	Disk     *disk.Disk
	Interval time.Duration
}

// Run starts with a wait: the bot has swept on its start.
func (s *Sweeper) Run(ctx context.Context) {
	every(ctx, s.Interval, nil, func(context.Context) { s.Disk.SweepScratch() })
}
