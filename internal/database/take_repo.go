package database

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/lubaskinc0de/navidrome-tg/internal/application"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type TakeRepository struct {
	db *gorm.DB
}

func NewTakeRepository(db *gorm.DB) *TakeRepository {
	return &TakeRepository{db: db}
}

func (r *TakeRepository) Save(ctx context.Context, take *domain.Take) error {
	return dbForContext(ctx, r.db).Omit("User", "Track").Create(take).Error
}

func (r *TakeRepository) TopTaken(ctx context.Context, since time.Time, limit int) ([]application.TopEntry, error) {
	query := dbForContext(ctx, r.db).
		Model(&domain.Take{}).
		Where("author_id IS NOT NULL AND author_id <> user_id AND created_at >= ?", since).
		Group("author_id")
	return topEntries(ctx, r.db, query, "author_id", limit)
}
