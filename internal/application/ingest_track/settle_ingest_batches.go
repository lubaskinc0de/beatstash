package ingest_track

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
)

// SettleIngestBatches finishes batches whose last job ended right before a crash.
type SettleIngestBatches struct {
	Tx       repositories.TxManager
	Queue    repositories.IngestQueue
	Batches  repositories.IngestBatches
	Reporter common.BatchReporter
}

func (i *SettleIngestBatches) Execute(ctx context.Context) error {
	batches, err := i.Batches.Unfinished(ctx)
	if err != nil {
		return err
	}
	for _, batch := range batches {
		if err := reportBatch(ctx, i.Tx, i.Queue, i.Batches, i.Reporter, batch.ID); err != nil {
			return err
		}
	}
	return nil
}
