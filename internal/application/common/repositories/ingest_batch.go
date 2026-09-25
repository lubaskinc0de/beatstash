package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type IngestBatches interface {
	Save(ctx context.Context, batch *domain.IngestBatch) error
	// GetForUpdate locks the batch until ctx's transaction ends.
	GetForUpdate(ctx context.Context, id uint) (*domain.IngestBatch, error)
	Unfinished(ctx context.Context) ([]domain.IngestBatch, error)
	Running(ctx context.Context, userID uint, provider domain.ProviderName, kind domain.IngestBatchKind) (bool, error)
	// Finish returns false if the batch was finished already.
	Finish(ctx context.Context, id uint, at time.Time) (bool, error)
}

var ErrBatchNotFound = errors.New("ingest batch not found")
