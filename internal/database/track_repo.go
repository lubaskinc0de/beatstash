package database

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/lubaskinc0de/navidrome-tg/internal/application"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type TrackRepository struct {
	db *gorm.DB
}

func NewTrackRepository(db *gorm.DB) *TrackRepository {
	return &TrackRepository{
		db: db,
	}
}

func (r *TrackRepository) FindSource(ctx context.Context, provider domain.ProviderName, ref string) (*domain.TrackSource, error) {
	var source domain.TrackSource

	err := dbForContext(ctx, r.db).Where("provider = ? AND ref = ?", provider, ref).First(&source).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, application.ErrTrackNotFound
		}
		return nil, err
	}
	return &source, nil
}

func (r *TrackRepository) FindDuplicate(
	ctx context.Context,
	m domain.Metadata,
	durationMs, toleranceMs int,
) (*domain.Track, error) {
	var track domain.Track

	err := dbForContext(ctx, r.db).
		Where("LOWER(artist) = LOWER(?)", m.Artist).
		Where("LOWER(title) = LOWER(?)", m.Title).
		Where("LOWER(album) = LOWER(?)", m.Album).
		Where("ABS(duration_ms - ?) <= ?", durationMs, toleranceMs).
		Order("id").
		First(&track).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, application.ErrTrackNotFound
		}
		return nil, err
	}
	return &track, nil
}

func (r *TrackRepository) SaveTrack(ctx context.Context, track *domain.Track) error {
	return dbForContext(ctx, r.db).Save(track).Error
}

func (r *TrackRepository) SaveSource(ctx context.Context, source *domain.TrackSource) error {
	return dbForContext(ctx, r.db).Omit("Track").Create(source).Error
}

func (r *TrackRepository) FindTelegramFile(ctx context.Context, m domain.Metadata) (*domain.TelegramFile, error) {
	var source domain.TrackSource

	// Prefer the same album, then files sendable as audio, then the oldest.
	err := dbForContext(ctx, r.db).
		Joins("JOIN tracks ON tracks.id = track_sources.track_id").
		Where("track_sources.telegram_file_id <> ''").
		Where("LOWER(tracks.artist) = LOWER(TRIM(?))", m.Artist).
		Where("LOWER(tracks.title) = LOWER(TRIM(?))", m.Title).
		Order(gorm.Expr("LOWER(tracks.album) = LOWER(TRIM(?)) DESC", m.Album)).
		Order(gorm.Expr("track_sources.telegram_file_kind = ? DESC", domain.TelegramFileAudio)).
		Order("track_sources.id").
		First(&source).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, application.ErrTrackNotFound
		}
		return nil, err
	}
	return source.TelegramFile(), nil
}
