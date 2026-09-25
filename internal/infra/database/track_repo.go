package database

import (
	"context"

	"gorm.io/gorm"

	"github.com/lubaskinc0de/navidrome-tg/internal/application"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type TrackRepository struct {
	DB *gorm.DB
}

func (r *TrackRepository) FindSource(
	ctx context.Context,
	libraryID uint,
	provider domain.ProviderName,
	ref string,
) (*domain.TrackSource, error) {
	q := dbForContext(ctx, r.DB).Where("library_id = ? AND provider = ? AND ref = ?", libraryID, provider, ref)
	return first[domain.TrackSource](q, application.ErrSourceNotFound)
}

func (r *TrackRepository) FindDuplicate(
	ctx context.Context,
	libraryID uint,
	m domain.Metadata,
	durationMs int,
) (*domain.Track, error) {
	q := dbForContext(ctx, r.DB).
		Where("library_id = ?", libraryID).
		Where("LOWER(artist) = LOWER(?)", m.Artist).
		Where("LOWER(title) = LOWER(?)", m.Title).
		Where("LOWER(album) = LOWER(?)", m.Album).
		Where("ABS(duration_ms - ?) <= ?", durationMs, domain.DuplicateToleranceMs).
		Order("id")
	return first[domain.Track](q, application.ErrTrackNotFound)
}

func (r *TrackRepository) SaveTrack(ctx context.Context, track *domain.Track) error {
	return dbForContext(ctx, r.DB).Save(track).Error
}

func (r *TrackRepository) SaveSource(ctx context.Context, source *domain.TrackSource) error {
	return dbForContext(ctx, r.DB).Omit("Track").Create(source).Error
}

func (r *TrackRepository) FindTelegramFile(ctx context.Context, libraryIDs []uint, m domain.Metadata) (*domain.TelegramFile, error) {
	// Prefer the same album, then files sendable as audio, then the oldest.
	source, err := first[domain.TrackSource](dbForContext(ctx, r.DB).
		Joins("JOIN tracks ON tracks.id = track_sources.track_id").
		Where("tracks.library_id IN ?", libraryIDs).
		Where("track_sources.telegram_file_id <> ''").
		Where("LOWER(tracks.artist) = LOWER(TRIM(?))", m.Artist).
		Where("LOWER(tracks.title) = LOWER(TRIM(?))", m.Title).
		Order(gorm.Expr("LOWER(tracks.album) = LOWER(TRIM(?)) DESC", m.Album)).
		Order(gorm.Expr("track_sources.telegram_file_kind = ? DESC", domain.TelegramFileAudio)).
		Order("track_sources.id"),
		application.ErrNoTelegramFile,
	)
	if err != nil {
		return nil, err
	}
	return source.TelegramFile(), nil
}

func (r *TrackRepository) Get(ctx context.Context, id uint) (*domain.Track, error) {
	return first[domain.Track](dbForContext(ctx, r.DB).Where("id = ?", id), application.ErrTrackNotFound)
}

func (r *TrackRepository) Sources(ctx context.Context, trackID uint) ([]domain.TrackSource, error) {
	var sources []domain.TrackSource
	err := dbForContext(ctx, r.DB).Where("track_id = ?", trackID).Order("id").Find(&sources).Error
	return sources, err
}

func (r *TrackRepository) Album(ctx context.Context, libraryID uint, albumArtist, album string) ([]domain.Track, error) {
	var tracks []domain.Track
	err := dbForContext(ctx, r.DB).
		Where("library_id = ?", libraryID).
		Where("LOWER(album_artist) = LOWER(?) AND LOWER(album) = LOWER(?)", albumArtist, album).
		Order("track_number, id").
		Find(&tracks).Error
	return tracks, err
}

func (r *TrackRepository) Delete(ctx context.Context, id uint) error {
	return dbForContext(ctx, r.DB).Delete(&domain.Track{}, id).Error
}

func (r *TrackRepository) TelegramFile(ctx context.Context, trackID uint) (*domain.TelegramFile, error) {
	q := dbForContext(ctx, r.DB).
		Where("track_id = ? AND telegram_file_id <> ''", trackID).
		Order(gorm.Expr("telegram_file_kind = ? DESC", domain.TelegramFileAudio)).
		Order("id")
	source, err := first[domain.TrackSource](q, application.ErrNoTelegramFile)
	if err != nil {
		return nil, err
	}
	return source.TelegramFile(), nil
}

func (r *TrackRepository) FindByMetadata(ctx context.Context, libraryID uint, m domain.Metadata) (*domain.Track, error) {
	q := dbForContext(ctx, r.DB).
		Where("library_id = ?", libraryID).
		Where("LOWER(artist) = LOWER(TRIM(?))", m.Artist).
		Where("LOWER(title) = LOWER(TRIM(?))", m.Title).
		Order(gorm.Expr("LOWER(album) = LOWER(TRIM(?)) DESC", m.Album)).
		Order("id")
	return first[domain.Track](q, application.ErrTrackNotFound)
}

func (r *TrackRepository) Count(ctx context.Context, libraryID uint) (int64, error) {
	var count int64
	err := dbForContext(ctx, r.DB).Model(&domain.Track{}).Where("library_id = ?", libraryID).Count(&count).Error
	return count, err
}
