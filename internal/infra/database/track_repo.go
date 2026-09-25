package database

import (
	"context"

	"gorm.io/gorm"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
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
	return first[domain.TrackSource](q, repositories.ErrSourceNotFound)
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
	return first[domain.Track](q, repositories.ErrTrackNotFound)
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
		Order(orderBy(
			"LOWER(tracks.album) = LOWER(TRIM(?)) DESC, track_sources.telegram_file_kind = ? DESC, track_sources.id",
			m.Album, domain.TelegramFileAudio,
		)),
		repositories.ErrNoTelegramFile,
	)
	if err != nil {
		return nil, err
	}
	return source.TelegramFile(), nil
}

func (r *TrackRepository) Get(ctx context.Context, id uint) (*domain.Track, error) {
	return first[domain.Track](dbForContext(ctx, r.DB).Where("id = ?", id), repositories.ErrTrackNotFound)
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
		Order(orderBy("telegram_file_kind = ? DESC, id", domain.TelegramFileAudio))
	source, err := first[domain.TrackSource](q, repositories.ErrNoTelegramFile)
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
		Order(orderBy("LOWER(album) = LOWER(TRIM(?)) DESC, id", m.Album))
	return first[domain.Track](q, repositories.ErrTrackNotFound)
}

func (r *TrackRepository) Count(ctx context.Context, libraryID uint) (int64, error) {
	var count int64
	err := dbForContext(ctx, r.DB).Model(&domain.Track{}).Where("library_id = ?", libraryID).Count(&count).Error
	return count, err
}

func (r *TrackRepository) Copies(ctx context.Context, trackID uint) ([]domain.Track, error) {
	var tracks []domain.Track
	err := dbForContext(ctx, r.DB).
		Where("id = ? OR id IN (?)", trackID, dbForContext(ctx, r.DB).
			Model(&domain.TrackSource{}).
			Select("copies.track_id").
			Joins("JOIN track_sources copies ON copies.provider = track_sources.provider AND copies.ref = track_sources.ref").
			Where("track_sources.track_id = ? AND track_sources.provider <> ?", trackID, domain.ProviderTelegram),
		).
		Order("id").
		Find(&tracks).Error
	return tracks, err
}

func (r *TrackRepository) KnownRefs(
	ctx context.Context,
	libraryID uint,
	provider domain.ProviderName,
	refs []string,
) ([]string, error) {
	var known []string
	err := dbForContext(ctx, r.DB).
		Model(&domain.TrackSource{}).
		Where("library_id = ? AND provider = ? AND ref IN ?", libraryID, provider, refs).
		Pluck("ref", &known).Error
	return known, err
}

func (r *TrackRepository) SourcePaths(
	ctx context.Context,
	libraryID uint,
	provider domain.ProviderName,
	refs []string,
) (map[string]string, error) {
	var rows []struct {
		Ref  string
		Path string
	}
	err := dbForContext(ctx, r.DB).
		Model(&domain.TrackSource{}).
		Select("track_sources.ref, tracks.path").
		Joins("JOIN tracks ON tracks.id = track_sources.track_id").
		Where("track_sources.library_id = ? AND track_sources.provider = ? AND track_sources.ref IN ?", libraryID, provider, refs).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	paths := make(map[string]string, len(rows))
	for _, row := range rows {
		paths[row.Ref] = row.Path
	}
	return paths, nil
}
