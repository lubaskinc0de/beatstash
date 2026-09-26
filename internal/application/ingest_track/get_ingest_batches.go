package ingest_track

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/ingest"
)

type BatchState struct {
	Batch    ingest.IngestBatch
	Progress repositories.BatchProgress
	// FailedNames are set once the batch is finished.
	FailedNames []string
}

func (s *BatchState) Finished() bool {
	return s.Batch.Finished()
}

// GetIngestBatches lets a Channel follow the progress of the batches it
// started; the system asks it, not a User.
type GetIngestBatches struct {
	Queue   repositories.IngestQueue
	Batches repositories.IngestBatches
}

// Execute leaves out batches that are gone, e.g. with their User.
func (i *GetIngestBatches) Execute(ctx context.Context, ids []uint) ([]BatchState, error) {
	states := make([]BatchState, 0, len(ids))
	for _, id := range ids {
		batch, err := i.Batches.Get(ctx, id)
		if errors.Is(err, repositories.ErrBatchNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		state := BatchState{Batch: *batch}
		if state.Progress, err = i.Queue.BatchProgress(ctx, id); err != nil {
			return nil, err
		}
		if state.Finished() {
			if state.FailedNames, err = i.Queue.FailedNames(ctx, id); err != nil {
				return nil, err
			}
		}
		states = append(states, state)
	}
	return states, nil
}
