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

func (r *TrackRepository) FindSource(
	ctx context.Context,
	libraryID uint,
	provider domain.ProviderName,
	ref string,
) (*domain.TrackSource, error) {
	var source domain.TrackSource

	err := dbForContext(ctx, r.db).
		Where("library_id = ? AND provider = ? AND ref = ?", libraryID, provider, ref).
		First(&source).Error
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
	libraryID uint,
	m domain.Metadata,
	durationMs int,
) (*domain.Track, error) {
	var track domain.Track

	err := dbForContext(ctx, r.db).
		Where("library_id = ?", libraryID).
		Where("LOWER(artist) = LOWER(?)", m.Artist).
		Where("LOWER(title) = LOWER(?)", m.Title).
		Where("LOWER(album) = LOWER(?)", m.Album).
		Where("ABS(duration_ms - ?) <= ?", durationMs, domain.DuplicateToleranceMs).
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

func (r *TrackRepository) FindTelegramFile(ctx context.Context, libraryIDs []uint, m domain.Metadata) (*domain.TelegramFile, error) {
	var source domain.TrackSource

	// Prefer the same album, then files sendable as audio, then the oldest.
	err := dbForContext(ctx, r.db).
		Joins("JOIN tracks ON tracks.id = track_sources.track_id").
		Where("tracks.library_id IN ?", libraryIDs).
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

func (r *TrackRepository) Get(ctx context.Context, id uint) (*domain.Track, error) {
	var track domain.Track
	err := dbForContext(ctx, r.db).First(&track, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, application.ErrTrackNotFound
	}
	if err != nil {
		return nil, err
	}
	return &track, nil
}

func (r *TrackRepository) Sources(ctx context.Context, trackID uint) ([]domain.TrackSource, error) {
	var sources []domain.TrackSource
	err := dbForContext(ctx, r.db).Where("track_id = ?", trackID).Order("id").Find(&sources).Error
	return sources, err
}

func (r *TrackRepository) Album(ctx context.Context, libraryID uint, albumArtist, album string) ([]domain.Track, error) {
	var tracks []domain.Track
	err := dbForContext(ctx, r.db).
		Where("library_id = ?", libraryID).
		Where("LOWER(album_artist) = LOWER(?) AND LOWER(album) = LOWER(?)", albumArtist, album).
		Order("track_number, id").
		Find(&tracks).Error
	return tracks, err
}

func (r *TrackRepository) Delete(ctx context.Context, id uint) error {
	return dbForContext(ctx, r.db).Delete(&domain.Track{}, id).Error
}

func (r *TrackRepository) TelegramFile(ctx context.Context, trackID uint) (*domain.TelegramFile, error) {
	var source domain.TrackSource
	err := dbForContext(ctx, r.db).
		Where("track_id = ? AND telegram_file_id <> ''", trackID).
		Order(gorm.Expr("telegram_file_kind = ? DESC", domain.TelegramFileAudio)).
		Order("id").
		First(&source).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, application.ErrTrackNotFound
	}
	if err != nil {
		return nil, err
	}
	return source.TelegramFile(), nil
}

func (r *TrackRepository) FindByMetadata(ctx context.Context, libraryID uint, m domain.Metadata) (*domain.Track, error) {
	var track domain.Track
	err := dbForContext(ctx, r.db).
		Where("library_id = ?", libraryID).
		Where("LOWER(artist) = LOWER(TRIM(?))", m.Artist).
		Where("LOWER(title) = LOWER(TRIM(?))", m.Title).
		Order(gorm.Expr("LOWER(album) = LOWER(TRIM(?)) DESC", m.Album)).
		Order("id").
		First(&track).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, application.ErrTrackNotFound
	}
	if err != nil {
		return nil, err
	}
	return &track, nil
}

func (r *TrackRepository) Count(ctx context.Context, libraryID uint) (int64, error) {
	var count int64
	err := dbForContext(ctx, r.db).Model(&domain.Track{}).Where("library_id = ?", libraryID).Count(&count).Error
	return count, err
}
