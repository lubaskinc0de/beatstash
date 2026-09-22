package database

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/application"
	"github.com/lubaskinc0de/navidrome-tg/internal/entities"
	"gorm.io/gorm"
)

type TrackRepository struct {
	db *gorm.DB
}

func NewTrackRepository(db *gorm.DB) *TrackRepository {
	return &TrackRepository{
		db: db,
	}
}

func (r *TrackRepository) Save(ctx context.Context, track *entities.Track) error {
	return dbForContext(ctx, r.db).Create(track).Error
}

func (r *TrackRepository) GetByUniqueId(ctx context.Context, uniqueId string) (*entities.Track, error) {
	var track entities.Track

	err := dbForContext(ctx, r.db).Where("telegram_file_unique_id = ?", uniqueId).First(&track).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, application.ErrTrackNotFound
		}
		return nil, err
	}
	return &track, nil
}