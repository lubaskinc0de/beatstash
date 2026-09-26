package ingest_track

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/ingest"
)

// GetIngestJobs lets a Channel follow the jobs it asked for; the system asks
// it, not a User.
type GetIngestJobs struct {
	Queue repositories.IngestQueue
}

// Execute leaves out jobs that are gone, e.g. with their User.
func (i *GetIngestJobs) Execute(ctx context.Context, ids []uint) ([]ingest.IngestJob, error) {
	return i.Queue.Get(ctx, ids)
}
