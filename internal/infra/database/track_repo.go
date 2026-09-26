package database

import (
	"context"

	"gorm.io/gorm"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
)

type TrackRepository struct {
	DB *gorm.DB
}

func (r *TrackRepository) FindSource(
	ctx context.Context,
	libraryID uint,
	providerName provider.ProviderName,
	ref string,
) (*library.TrackSource, error) {
	q := dbForContext(ctx, r.DB).Where("library_id = ? AND provider = ? AND ref = ?", libraryID, providerName, ref)
	return first[library.TrackSource](q, repositories.ErrSourceNotFound)
}

func (r *TrackRepository) FindDuplicate(
	ctx context.Context,
	libraryID uint,
	m library.Metadata,
	durationMs int,
) (*library.Track, error) {
	if !m.Complete() {
		return nil, repositories.ErrTrackNotFound
	}
	q := tracks(ctx, r.DB).
		Where("library_id = ?", libraryID).
		Where("LOWER(artist) = LOWER(?)", m.Artist).
		Where("LOWER(title) = LOWER(?)", m.Title).
		Where("LOWER(album) = LOWER(?)", m.Album).
		Where("ABS(duration_ms - ?) <= ?", durationMs, library.DuplicateToleranceMs).
		Order("id")
	return first[library.Track](q, repositories.ErrTrackNotFound)
}

func (r *TrackRepository) SaveTrack(ctx context.Context, track *library.Track) error {
	return dbForContext(ctx, r.DB).Save(track).Error
}

func (r *TrackRepository) Get(ctx context.Context, id uint) (*library.Track, error) {
	return first[library.Track](tracks(ctx, r.DB).Where("id = ?", id), repositories.ErrTrackNotFound)
}

func (r *TrackRepository) Album(ctx context.Context, libraryID uint, albumArtist, albumTitle string) ([]library.Track, error) {
	var album []library.Track
	err := tracks(ctx, r.DB).
		Where("library_id = ?", libraryID).
		Where("LOWER(album_artist) = LOWER(?) AND LOWER(album) = LOWER(?)", albumArtist, albumTitle).
		Order("track_number, id").
		Find(&album).Error
	return album, err
}

func (r *TrackRepository) FindByMetadata(ctx context.Context, libraryIDs []uint, m library.Metadata) (*library.Track, error) {
	q := tracks(ctx, r.DB).
		Where("library_id IN ?", libraryIDs).
		Where("LOWER(artist) = LOWER(TRIM(?))", m.Artist).
		Where("LOWER(title) = LOWER(TRIM(?))", m.Title).
		Order(orderBy("LOWER(album) = LOWER(TRIM(?)) DESC, id", m.Album))
	return first[library.Track](q, repositories.ErrTrackNotFound)
}

func (r *TrackRepository) Count(ctx context.Context, libraryID uint) (int64, error) {
	var count int64
	err := dbForContext(ctx, r.DB).Model(&library.Track{}).Where("library_id = ?", libraryID).Count(&count).Error
	return count, err
}

func (r *TrackRepository) KnownRefs(
	ctx context.Context,
	libraryID uint,
	providerName provider.ProviderName,
	refs []string,
) ([]string, error) {
	var known []string
	err := dbForContext(ctx, r.DB).
		Model(&library.TrackSource{}).
		Where("library_id = ? AND provider = ? AND ref IN ?", libraryID, providerName, refs).
		Pluck("ref", &known).Error
	return known, err
}

func (r *TrackRepository) SourcePaths(
	ctx context.Context,
	libraryID uint,
	providerName provider.ProviderName,
	refs []string,
) (map[string]string, error) {
	var rows []struct {
		Ref  string
		Path string
	}
	err := dbForContext(ctx, r.DB).
		Model(&library.TrackSource{}).
		Select("track_sources.ref, tracks.path").
		Joins("JOIN tracks ON tracks.id = track_sources.track_id").
		Where("track_sources.library_id = ? AND track_sources.provider = ? AND track_sources.ref IN ?", libraryID, providerName, refs).
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

// tracks loads Tracks whole, with their Sources.
func tracks(ctx context.Context, db *gorm.DB) *gorm.DB {
	return dbForContext(ctx, db).Preload("Sources", func(q *gorm.DB) *gorm.DB { return q.Order("id") })
}
