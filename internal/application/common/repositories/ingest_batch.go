package repositories

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/beatstash/internal/domain/ingest"
	"github.com/lubaskinc0de/beatstash/internal/domain/provider"
)

type IngestBatches interface {
	Save(ctx context.Context, batch *ingest.IngestBatch) error
	// GetOf leaves out the batches that are not the user's.
	GetOf(ctx context.Context, userID uint, ids []uint) ([]ingest.IngestBatch, error)
	GetForUpdate(ctx context.Context, id uint) (*ingest.IngestBatch, error)
	Unfinished(ctx context.Context) ([]ingest.IngestBatch, error)
	UnfinishedOf(ctx context.Context, userID uint) ([]ingest.IngestBatch, error)
	Running(ctx context.Context, userID uint, providerName provider.ProviderName, kind ingest.IngestBatchKind) (bool, error)
}

var ErrBatchNotFound = errors.New("ingest batch not found")
