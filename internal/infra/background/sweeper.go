package background

import (
	"context"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/infra/disk"
)

// Sweeper removes the scratch files crashed Ingests left behind.
type Sweeper struct {
	Disk     *disk.Disk
	Interval time.Duration
}

// Run starts with a wait: the bot has swept on its start.
func (s *Sweeper) Run(ctx context.Context) {
	every(ctx, s.Interval, nil, func(context.Context) { s.Disk.SweepScratch() })
}
