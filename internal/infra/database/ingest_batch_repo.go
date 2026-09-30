package database

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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

func (r *IngestBatchRepository) GetOf(ctx context.Context, userID uint, ids []uint) ([]ingest.IngestBatch, error) {
	var batches []ingest.IngestBatch
	err := dbForContext(ctx, r.DB).Where("user_id = ? AND id IN ?", userID, ids).Order("id").Find(&batches).Error
	return batches, err
}

func (r *IngestBatchRepository) Unfinished(ctx context.Context) ([]ingest.IngestBatch, error) {
	var batches []ingest.IngestBatch
	err := dbForContext(ctx, r.DB).Where("finished_at IS NULL").Order("id").Find(&batches).Error
	return batches, err
}

func (r *IngestBatchRepository) UnfinishedOf(ctx context.Context, userID uint) ([]ingest.IngestBatch, error) {
	var batches []ingest.IngestBatch
	err := dbForContext(ctx, r.DB).Where("user_id = ? AND finished_at IS NULL", userID).Order("id").Find(&batches).Error
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

func (r *IngestBatchRepository) GetForUpdate(ctx context.Context, id uint) (*ingest.IngestBatch, error) {
	q := dbForContext(ctx, r.DB).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id)
	return first[ingest.IngestBatch](q, repositories.ErrBatchNotFound)
}
