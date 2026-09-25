package database

import (
	"context"

	"gorm.io/gorm"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type UploadRepository struct {
	db *gorm.DB
}

func NewUploadRepository(db *gorm.DB) *UploadRepository {
	return &UploadRepository{db: db}
}

func (r *UploadRepository) Save(ctx context.Context, upload *domain.Upload) error {
	return dbForContext(ctx, r.db).Omit("User", "Track", "TrackSource").Create(upload).Error
}
