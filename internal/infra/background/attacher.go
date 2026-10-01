package background

import (
	"context"
	"log/slog"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/attach_libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/runs"
)

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
