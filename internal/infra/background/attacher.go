package background

import (
	"context"
	"log/slog"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/attach_libraries"
)

// Attacher keeps the Attached Libraries in step with Navidrome. A zero
// Interval turns them off.
type Attacher struct {
	Attach   *attach_libraries.AttachLibraries
	Interval time.Duration
}

// Once is not fatal: Navidrome may be down, and the next run catches up.
func (a *Attacher) Once(ctx context.Context) {
	if a.Interval <= 0 {
		return
	}
	if err := a.Attach.Execute(ctx); err != nil && ctx.Err() == nil {
		slog.Error("attach_libraries", "error", err)
	}
}

// Run starts with a wait: the bot has attached on its start.
func (a *Attacher) Run(ctx context.Context) {
	if a.Interval <= 0 {
		return
	}
	ticker := time.NewTicker(a.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.Once(ctx)
		}
	}
}
