package database

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type IngestBatchRepository struct {
	DB *gorm.DB
}

func (r *IngestBatchRepository) Save(ctx context.Context, batch *domain.IngestBatch) error {
	return dbForContext(ctx, r.DB).Omit("User").Save(batch).Error
}

func (r *IngestBatchRepository) Get(ctx context.Context, id uint) (*domain.IngestBatch, error) {
	return first[domain.IngestBatch](dbForContext(ctx, r.DB).Where("id = ?", id), repositories.ErrBatchNotFound)
}

func (r *IngestBatchRepository) GetForUpdate(ctx context.Context, id uint) (*domain.IngestBatch, error) {
	q := dbForContext(ctx, r.DB).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id)
	return first[domain.IngestBatch](q, repositories.ErrBatchNotFound)
}

func (r *IngestBatchRepository) Unfinished(ctx context.Context) ([]domain.IngestBatch, error) {
	var batches []domain.IngestBatch
	err := dbForContext(ctx, r.DB).Where("finished_at IS NULL").Order("id").Find(&batches).Error
	return batches, err
}

func (r *IngestBatchRepository) Running(
	ctx context.Context,
	userID uint,
	provider domain.ProviderName,
	kind domain.IngestBatchKind,
) (bool, error) {
	var count int64
	err := dbForContext(ctx, r.DB).
		Model(&domain.IngestBatch{}).
		Where("user_id = ? AND provider = ? AND kind = ? AND finished_at IS NULL", userID, provider, kind).
		Count(&count).Error
	return count > 0, err
}

func (r *IngestBatchRepository) Finish(ctx context.Context, id uint, at time.Time) (bool, error) {
	result := dbForContext(ctx, r.DB).
		Model(&domain.IngestBatch{}).
		Where("id = ? AND finished_at IS NULL", id).
		Update("finished_at", at)
	return result.RowsAffected == 1, result.Error
}
