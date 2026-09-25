package database

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/lubaskinc0de/navidrome-tg/internal/application"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type ShareRepository struct {
	db *gorm.DB
}

func NewShareRepository(db *gorm.DB) *ShareRepository {
	return &ShareRepository{db: db}
}

func (r *ShareRepository) Save(ctx context.Context, share *domain.Share) error {
	return dbForContext(ctx, r.db).Omit("Track", "SourceTrack", "User").Create(share).Error
}

func (r *ShareRepository) BySource(ctx context.Context, sourceTrackID uint) (*domain.Share, error) {
	var share domain.Share
	err := dbForContext(ctx, r.db).Where("source_track_id = ?", sourceTrackID).First(&share).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, application.ErrShareNotFound
	}
	if err != nil {
		return nil, err
	}
	return &share, nil
}

func (r *ShareRepository) ForTrack(ctx context.Context, trackID uint) ([]domain.Share, error) {
	var shares []domain.Share
	err := dbForContext(ctx, r.db).
		Preload("User").
		Where("track_id = ?", trackID).
		Order("created_at, id").
		Find(&shares).Error
	return shares, err
}

func (r *ShareRepository) Delete(ctx context.Context, id uint) error {
	return dbForContext(ctx, r.db).Delete(&domain.Share{}, id).Error
}

// Feed takes the oldest Share of each Track: its sharer is the author.
func (r *ShareRepository) Feed(ctx context.Context, limit int) ([]domain.Share, error) {
	var shares []domain.Share
	err := dbForContext(ctx, r.db).
		Preload("User").
		Preload("Track").
		Where(`NOT EXISTS (
			SELECT 1 FROM shares earlier
			WHERE earlier.track_id = shares.track_id
			AND (earlier.created_at, earlier.id) < (shares.created_at, shares.id)
		)`).
		Order("created_at DESC, id DESC").
		Limit(limit).
		Find(&shares).Error
	return shares, err
}

func (r *ShareRepository) TopSharers(ctx context.Context, since time.Time, limit int) ([]application.TopEntry, error) {
	query := dbForContext(ctx, r.db).
		Model(&domain.Share{}).
		Where("in_top AND created_at >= ?", since).
		Group("user_id")
	return topEntries(ctx, r.db, query, "user_id", limit)
}

// topEntries ranks by count; a tie goes to whoever got there first.
func topEntries(ctx context.Context, db *gorm.DB, grouped *gorm.DB, userColumn string, limit int) ([]application.TopEntry, error) {
	var rows []struct {
		UserID uint
		Count  int
	}
	err := grouped.
		Select(userColumn + " AS user_id, COUNT(*) AS count").
		Order("count DESC, MIN(created_at)").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	entries := make([]application.TopEntry, 0, len(rows))
	for _, row := range rows {
		var user domain.User
		if err := dbForContext(ctx, db).First(&user, row.UserID).Error; err != nil {
			return nil, err
		}
		entries = append(entries, application.TopEntry{User: user, Count: row.Count})
	}
	return entries, nil
}
