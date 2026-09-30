package add_track

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/ingest"
)

var ErrIngestJobNotFound = errors.New("ingest job not found")

// GetIngestResults tells the User what became of the tracks they sent.
type GetIngestResults struct {
	IDs   common.IDProvider
	Queue repositories.IngestQueue
}

// Execute fails with ErrIngestJobNotFound if any of the jobs is not the
// User's.
func (i *GetIngestResults) Execute(ctx context.Context, ids []uint) ([]ingest.IngestJob, error) {
	user, err := i.IDs.CurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	jobs, err := i.Queue.GetOf(ctx, user.ID, ids)
	if err != nil {
		return nil, err
	}
	if len(jobs) != len(ids) {
		return nil, ErrIngestJobNotFound
	}
	return jobs, nil
}
