package add_track

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/providers"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type IngestRequest struct {
	Ref     domain.TrackRef
	Message common.MessageRef
}

type EnqueueIngest struct {
	IDs   common.IDProvider
	Queue repositories.IngestQueue
	Waker common.Waker
}

func (i *EnqueueIngest) Execute(ctx context.Context, req IngestRequest) error {
	user, err := i.IDs.CurrentUser(ctx)
	if err != nil {
		return err
	}

	job := common.NewJob(user.ID, providers.ListedTrack{Ref: req.Ref}, req.Message)
	if err := i.Queue.Enqueue(ctx, job); err != nil {
		return err
	}

	i.Waker.Wake()
	return nil
}
