package ingest_track

import (
	"context"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
)

// reportBatch locks the batch row: a late progress must not follow the summary.
func reportBatch(
	ctx context.Context,
	tx repositories.TxManager,
	queue repositories.IngestQueue,
	batches repositories.IngestBatches,
	reporter common.BatchReporter,
	batchID uint,
) error {
	return tx.WithinTx(ctx, func(ctx context.Context) error {
		batch, err := batches.GetForUpdate(ctx, batchID)
		if err != nil || batch.FinishedAt != nil {
			return err
		}
		progress, err := queue.BatchProgress(ctx, batchID)
		if err != nil {
			return err
		}
		if progress.Pending > 0 {
			reporter.Progress(ctx, batch, progress)
			return nil
		}

		if _, err := batches.Finish(ctx, batchID, time.Now()); err != nil {
			return err
		}
		failed, err := queue.FailedNames(ctx, batchID)
		if err != nil {
			return err
		}
		reporter.Finished(ctx, batch, progress, failed)
		return nil
	})
}
