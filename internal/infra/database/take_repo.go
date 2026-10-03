package database

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/lubaskinc0de/beatstash/internal/application/common/repositories"
	"github.com/lubaskinc0de/beatstash/internal/domain/sharing"
)

type TakeRepository struct {
	DB *gorm.DB
}

func (r *TakeRepository) Save(ctx context.Context, take *sharing.Take) error {
	return dbForContext(ctx, r.DB).Omit("User", "Track").Create(take).Error
}

func (r *TakeRepository) TopTaken(ctx context.Context, since time.Time, limit int) ([]repositories.TopEntry, error) {
	query := dbForContext(ctx, r.DB).
		Model(&sharing.Take{}).
		Where("author_id IS NOT NULL AND author_id <> user_id AND created_at >= ?", since).
		Group("author_id")
	return topEntries(ctx, r.DB, query, "author_id", limit)
}

func (r *TakeRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := dbForContext(ctx, r.DB).Model(&sharing.Take{}).Count(&count).Error
	return count, err
}
