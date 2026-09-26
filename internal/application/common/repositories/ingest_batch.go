package repositories

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/ingest"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
)

type IngestBatches interface {
	Save(ctx context.Context, batch *ingest.IngestBatch) error
	Get(ctx context.Context, id uint) (*ingest.IngestBatch, error)
	GetForUpdate(ctx context.Context, id uint) (*ingest.IngestBatch, error)
	Unfinished(ctx context.Context) ([]ingest.IngestBatch, error)
	Running(ctx context.Context, userID uint, providerName provider.ProviderName, kind ingest.IngestBatchKind) (bool, error)
}

var ErrBatchNotFound = errors.New("ingest batch not found")
