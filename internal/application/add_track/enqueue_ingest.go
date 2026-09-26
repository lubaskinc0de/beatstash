package add_track

import (
	"context"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/ingest"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
)

type EnqueueIngest struct {
	IDs   common.IDProvider
	Queue repositories.IngestQueue
	Waker common.Waker
	Clock func() time.Time
}

// Execute returns the job's id: the Channel follows the job by it.
func (i *EnqueueIngest) Execute(ctx context.Context, ref provider.TrackRef) (uint, error) {
	user, err := i.IDs.CurrentUser(ctx)
	if err != nil {
		return 0, err
	}

	job := ingest.NewJob(user.ID, ref, "", i.Clock())
	if err := i.Queue.Enqueue(ctx, job); err != nil {
		return 0, err
	}

	i.Waker.Wake()
	return job.ID, nil
}
