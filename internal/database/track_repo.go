package database

import (
	"context"
	"errors"
	"strings"

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
func (r *TrackRepository) FindByTitleAndPerformer(
	ctx context.Context,
	title string,
	performer string,
) (*entities.Track, error) {
	if strings.TrimSpace(title) == "" {
		return nil, application.ErrTrackNotFound
	}

	db := dbForContext(ctx, r.db)
	var track entities.Track

	err := db.
		Where("LOWER(TRIM(title)) = LOWER(TRIM(?))", title).
		Where("LOWER(TRIM(performer)) = LOWER(TRIM(?))", performer).
		Order("created_at DESC").
		First(&track).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = db.
			Where("LOWER(TRIM(title)) = LOWER(TRIM(?))", title).
			Order("created_at DESC").
			First(&track).Error
	}

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, application.ErrTrackNotFound
		}
		return nil, err
	}
	return &track, nil
}
