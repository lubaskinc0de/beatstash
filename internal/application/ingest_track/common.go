package ingest_track

import (
	"context"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
)

// finishBatch finishes the batch once none of its jobs is pending. The row
// lock keeps two workers ending the last jobs from both finishing it.
func finishBatch(
	ctx context.Context,
	tx repositories.TxManager,
	queue repositories.IngestQueue,
	batches repositories.IngestBatches,
	batchID uint,
) error {
	return tx.WithinTx(ctx, func(ctx context.Context) error {
		batch, err := batches.GetForUpdate(ctx, batchID)
		if err != nil {
			return err
		}
		progress, err := queue.BatchProgress(ctx, []uint{batchID})
		if err != nil || progress[batchID].Pending > 0 {
			return err
		}
		if !batch.Finish(time.Now()) {
			return nil
		}
		return batches.Save(ctx, batch)
	})
}
