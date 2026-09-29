package ingest_track

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/ingest"
)

type BatchState struct {
	Batch    ingest.IngestBatch
	Progress repositories.BatchProgress
	// FailedNames are set once the batch is finished.
	FailedNames []string
}

// GetIngestBatches lets a Channel follow the progress of the batches it
// started; the system asks it, not a User.
type GetIngestBatches struct {
	Queue   repositories.IngestQueue
	Batches repositories.IngestBatches
}

// Execute leaves out batches that are gone, e.g. with their User.
func (i *GetIngestBatches) Execute(ctx context.Context, ids []uint) ([]BatchState, error) {
	batches, err := i.Batches.Get(ctx, ids)
	if err != nil {
		return nil, err
	}
	progress, err := i.Queue.BatchProgress(ctx, ids)
	if err != nil {
		return nil, err
	}
	var finished []uint
	for _, batch := range batches {
		if batch.Finished() {
			finished = append(finished, batch.ID)
		}
	}
	failed := map[uint][]string{}
	if len(finished) > 0 {
		if failed, err = i.Queue.FailedNames(ctx, finished); err != nil {
			return nil, err
		}
	}

	states := make([]BatchState, 0, len(batches))
	for _, batch := range batches {
		states = append(states, BatchState{Batch: batch, Progress: progress[batch.ID], FailedNames: failed[batch.ID]})
	}
	return states, nil
}
