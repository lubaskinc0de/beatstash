package background

import (
	"context"
	"log/slog"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/reconcile_libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/runs"
)

// Reconciler keeps the Tracks in step with the files of the Libraries.
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
