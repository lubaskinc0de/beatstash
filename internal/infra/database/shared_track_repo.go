package database

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/sharing"
)

type SharedTrackRepository struct {
	DB *gorm.DB
}

func (r *SharedTrackRepository) Get(ctx context.Context, trackID uint) (*sharing.SharedTrack, error) {
	db := dbForContext(ctx, r.DB)
	track, err := first[library.Track](tracks(ctx, r.DB).Where("id = ?", trackID), sharing.ErrNotShared)
	if err != nil {
		return nil, err
	}
	var shares []sharing.Share
	err = db.Preload("User").Where("track_id = ?", trackID).Order("created_at, id").Find(&shares).Error
	if err != nil {
		return nil, err
	}
	if len(shares) == 0 {
		return nil, sharing.ErrNotShared
	}
	return &sharing.SharedTrack{Track: track, Shares: shares}, nil
}

func (r *SharedTrackRepository) BySource(ctx context.Context, sourceTrackID uint) (*sharing.SharedTrack, error) {
	q := dbForContext(ctx, r.DB).Where("source_track_id = ?", sourceTrackID)
	share, err := first[sharing.Share](q, sharing.ErrNotShared)
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, share.TrackID)
}

func (r *SharedTrackRepository) SharedSources(ctx context.Context, sourceTrackIDs []uint) (map[uint]bool, error) {
	var ids []uint
	err := dbForContext(ctx, r.DB).
		Model(&sharing.Share{}).
		Where("source_track_id IN ?", sourceTrackIDs).
		Pluck("source_track_id", &ids).Error
	shared := make(map[uint]bool, len(ids))
	for _, id := range ids {
		shared[id] = true
	}
	return shared, err
}

func (r *SharedTrackRepository) Save(ctx context.Context, shared *sharing.SharedTrack) error {
	db := dbForContext(ctx, r.DB)
	kept := []uint{0}
	for _, share := range shared.Shares {
		kept = append(kept, share.ID)
	}
	err := db.Where("track_id = ? AND id NOT IN ?", shared.Track.ID, kept).Delete(&sharing.Share{}).Error
	if err != nil {
		return err
	}
	for n := range shared.Shares {
		share := &shared.Shares[n]
		if share.ID != 0 {
			continue
		}
		if err := db.Omit("Track", "SourceTrack", "User").Create(share).Error; err != nil {
			return err
		}
	}
	return nil
}

// Delete relies on the Shares cascading with their Track.
func (r *SharedTrackRepository) Delete(ctx context.Context, shared *sharing.SharedTrack) error {
	return dbForContext(ctx, r.DB).Delete(&library.Track{}, shared.Track.ID).Error
}

// Feed takes the oldest Share of each Track: its sharer is the author.
func (r *SharedTrackRepository) Feed(ctx context.Context, limit int) ([]sharing.Share, error) {
	var shares []sharing.Share
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

func (r *SharedTrackRepository) TopSharers(ctx context.Context, since time.Time, limit int) ([]repositories.TopEntry, error) {
	query := dbForContext(ctx, r.DB).
		Model(&sharing.Share{}).
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
	var users []access.User
	if err := dbForContext(ctx, db).Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, err
	}
	byID := make(map[uint]access.User, len(users))
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
