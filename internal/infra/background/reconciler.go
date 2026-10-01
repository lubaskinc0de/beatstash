package background

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/reconcile_libraries"
)

// Reconciler keeps the Tracks in step with the files of the Libraries.
type Reconciler struct {
	Reconcile *reconcile_libraries.ReconcileLibraries
	Interval  time.Duration

	mu sync.Mutex
	// next closes when the next run to start is over.
	next chan struct{}
}

func (r *Reconciler) Once(ctx context.Context) {
	r.mu.Lock()
	done := r.nextRun()
	r.next = nil
	r.mu.Unlock()
	defer close(done)

	if err := r.Reconcile.Execute(ctx); err != nil && ctx.Err() == nil {
		slog.Error("reconcile_libraries", "error", err)
	}
}

// Run starts with a wait: the bot has reconciled on its start.
func (r *Reconciler) Run(ctx context.Context) {
	every(ctx, r.Interval, r.Once)
}

// Wait blocks until a run that starts after the call is over; it lets a
// caller see the effect of files changed by hand.
func (r *Reconciler) Wait(ctx context.Context) error {
	r.mu.Lock()
	done := r.nextRun()
	r.mu.Unlock()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *Reconciler) nextRun() chan struct{} {
	if r.next == nil {
		r.next = make(chan struct{})
	}
	return r.next
}
