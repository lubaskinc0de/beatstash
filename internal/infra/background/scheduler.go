package background

import (
	"context"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/sync_collection"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
)

type Scheduler struct {
	Provider provider.ProviderName
	Sync     *sync_collection.SyncCollection
	Tick     time.Duration
}

func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.Tick)
	defer ticker.Stop()

	for {
		s.Sync.Execute(ctx, s.Provider)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
