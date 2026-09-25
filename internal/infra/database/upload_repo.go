package database

import (
	"context"

	"gorm.io/gorm"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type UploadRepository struct {
	DB *gorm.DB
}

func (r *UploadRepository) Save(ctx context.Context, upload *domain.Upload) error {
	return dbForContext(ctx, r.DB).Omit("User", "Track", "TrackSource").Create(upload).Error
}
