package database

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/ingest"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
)

type IngestBatchRepository struct {
	DB *gorm.DB
}

func (r *IngestBatchRepository) Save(ctx context.Context, batch *ingest.IngestBatch) error {
	return dbForContext(ctx, r.DB).Omit("User").Save(batch).Error
}

func (r *IngestBatchRepository) Get(ctx context.Context, id uint) (*ingest.IngestBatch, error) {
	return first[ingest.IngestBatch](dbForContext(ctx, r.DB).Where("id = ?", id), repositories.ErrBatchNotFound)
}

func (r *IngestBatchRepository) Unfinished(ctx context.Context) ([]ingest.IngestBatch, error) {
	var batches []ingest.IngestBatch
	err := dbForContext(ctx, r.DB).Where("finished_at IS NULL").Order("id").Find(&batches).Error
	return batches, err
}

func (r *IngestBatchRepository) Running(
	ctx context.Context,
	userID uint,
	providerName provider.ProviderName,
	kind ingest.IngestBatchKind,
) (bool, error) {
	var count int64
	err := dbForContext(ctx, r.DB).
		Model(&ingest.IngestBatch{}).
		Where("user_id = ? AND provider = ? AND kind = ? AND finished_at IS NULL", userID, providerName, kind).
		Count(&count).Error
	return count > 0, err
}

func (r *IngestBatchRepository) Finish(ctx context.Context, id uint, at time.Time) (bool, error) {
	result := dbForContext(ctx, r.DB).
		Model(&ingest.IngestBatch{}).
		Where("id = ? AND finished_at IS NULL", id).
		Update("finished_at", at)
	return result.RowsAffected == 1, result.Error
}
