package background

import (
	"context"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/sync_collection"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/runs"
)

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
