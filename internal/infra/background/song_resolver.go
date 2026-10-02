package background

import (
	"context"
	"log/slog"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/resolve_songs"
)

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
