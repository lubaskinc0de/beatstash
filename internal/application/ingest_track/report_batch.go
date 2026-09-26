package ingest_track

import (
	"context"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
)

func finishBatch(
	ctx context.Context,
	queue repositories.IngestQueue,
	batches repositories.IngestBatches,
	batchID uint,
) error {
	progress, err := queue.BatchProgress(ctx, batchID)
	if err != nil || progress.Pending > 0 {
		return err
	}
	_, err = batches.Finish(ctx, batchID, time.Now())
	return err
}
