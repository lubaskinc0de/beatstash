package database

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type ShareRepository struct {
	DB *gorm.DB
}

func (r *ShareRepository) Save(ctx context.Context, share *domain.Share) error {
	return dbForContext(ctx, r.DB).Omit("Track", "SourceTrack", "User").Create(share).Error
}

func (r *ShareRepository) BySource(ctx context.Context, sourceTrackID uint) (*domain.Share, error) {
	q := dbForContext(ctx, r.DB).Where("source_track_id = ?", sourceTrackID)
	return first[domain.Share](q, repositories.ErrShareNotFound)
}

func (r *ShareRepository) ForTrack(ctx context.Context, trackID uint) ([]domain.Share, error) {
	var shares []domain.Share
	err := dbForContext(ctx, r.DB).
		Preload("User").
		Where("track_id = ?", trackID).
		Order("created_at, id").
		Find(&shares).Error
	return shares, err
}

func (r *ShareRepository) Delete(ctx context.Context, id uint) error {
	return dbForContext(ctx, r.DB).Delete(&domain.Share{}, id).Error
}

// Feed takes the oldest Share of each Track: its sharer is the author.
func (r *ShareRepository) Feed(ctx context.Context, limit int) ([]domain.Share, error) {
	var shares []domain.Share
	err := dbForContext(ctx, r.DB).
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

func (r *ShareRepository) TopSharers(ctx context.Context, since time.Time, limit int) ([]repositories.TopEntry, error) {
	query := dbForContext(ctx, r.DB).
		Model(&domain.Share{}).
		Where("in_top AND created_at >= ?", since).
		Group("user_id")
	return topEntries(ctx, r.DB, query, "user_id", limit)
}

// topEntries ranks by count; a tie goes to whoever got there first.
func topEntries(ctx context.Context, db *gorm.DB, grouped *gorm.DB, userColumn string, limit int) ([]repositories.TopEntry, error) {
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

	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.UserID)
	}
	var users []domain.User
	if err := dbForContext(ctx, db).Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, err
	}
	byID := make(map[uint]domain.User, len(users))
	for _, user := range users {
		byID[user.ID] = user
	}

	entries := make([]repositories.TopEntry, 0, len(rows))
	for _, row := range rows {
		user, ok := byID[row.UserID]
		if !ok {
			return nil, fmt.Errorf("top: user %d not found", row.UserID)
		}
		entries = append(entries, repositories.TopEntry{User: user, Count: row.Count})
	}
	return entries, nil
}
