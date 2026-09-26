package ingest_track

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
)

// SettleIngestBatches finishes batches whose last job ended right before a crash.
type SettleIngestBatches struct {
	Tx      repositories.TxManager
	Queue   repositories.IngestQueue
	Batches repositories.IngestBatches
}

func (i *SettleIngestBatches) Execute(ctx context.Context) error {
	batches, err := i.Batches.Unfinished(ctx)
	if err != nil {
		return err
	}
	for _, batch := range batches {
		if err := finishBatch(ctx, i.Tx, i.Queue, i.Batches, batch.ID); err != nil {
			return err
		}
	}
	return nil
}
