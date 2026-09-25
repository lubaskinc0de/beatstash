package database

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type TakeRepository struct {
	DB *gorm.DB
}

func (r *TakeRepository) Save(ctx context.Context, take *domain.Take) error {
	return dbForContext(ctx, r.DB).Omit("User", "Track").Create(take).Error
}

func (r *TakeRepository) TopTaken(ctx context.Context, since time.Time, limit int) ([]repositories.TopEntry, error) {
	query := dbForContext(ctx, r.DB).
		Model(&domain.Take{}).
		Where("author_id IS NOT NULL AND author_id <> user_id AND created_at >= ?", since).
		Group("author_id")
	return topEntries(ctx, r.DB, query, "author_id", limit)
}
