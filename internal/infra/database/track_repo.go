package database

import (
	"context"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
)

type TrackRepository struct {
	DB *gorm.DB
}

func (r *TrackRepository) FindSource(
	ctx context.Context,
	libraryIDs []uint,
	providerName provider.ProviderName,
	ref string,
) (*library.TrackSource, error) {
	q := dbForContext(ctx, r.DB).Where("library_id IN ? AND provider = ? AND ref = ?", libraryIDs, providerName, ref)
	return first[library.TrackSource](q, repositories.ErrSourceNotFound)
}

func (r *TrackRepository) FindDuplicate(
	ctx context.Context,
	libraryIDs []uint,
	m library.Metadata,
	durationMs int,
) (*library.Track, error) {
	if !m.Complete() || len(libraryIDs) == 0 {
		return nil, repositories.ErrTrackNotFound
	}
	q := tracks(ctx, r.DB).
		Where("library_id IN ?", libraryIDs).
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

// saveBatch keeps a statement within Postgres' limit of parameters.
const saveBatch = 500

func (r *TrackRepository) SaveTracks(ctx context.Context, tracks []*library.Track) error {
	if len(tracks) == 0 {
		return nil
	}
	return dbForContext(ctx, r.DB).Clauses(clause.OnConflict{UpdateAll: true}).CreateInBatches(tracks, saveBatch).Error
}

func (r *TrackRepository) Get(ctx context.Context, id uint) (*library.Track, error) {
	return first[library.Track](tracks(ctx, r.DB).Where("id = ?", id), repositories.ErrTrackNotFound)
}

func (r *TrackRepository) GetMany(ctx context.Context, ids []uint) ([]library.Track, error) {
	var found []library.Track
	err := tracks(ctx, r.DB).Where("id IN ?", ids).Order("id").Find(&found).Error
	return found, err
}

// WithDuplicates checks all the tracks in one query, by the same rule as
// FindDuplicate.
func (r *TrackRepository) WithDuplicates(ctx context.Context, libraryIDs []uint, candidates []library.Track) (map[uint]bool, error) {
	has := map[uint]bool{}
	var values []string
	var args []any
	for _, t := range candidates {
		if t.Complete() {
			values = append(values, "(?::bigint, ?::text, ?::text, ?::text, ?::int)")
			args = append(args, t.ID, t.Artist, t.Title, t.Album, t.DurationMs)
		}
	}
	if len(values) == 0 || len(libraryIDs) == 0 {
		return has, nil
	}
	args = append(args, libraryIDs, library.DuplicateToleranceMs)

	var ids []uint
	err := dbForContext(ctx, r.DB).Raw(`
		SELECT c.id FROM (VALUES `+strings.Join(values, ", ")+`) AS c(id, artist, title, album, duration_ms)
		WHERE EXISTS (
			SELECT 1 FROM tracks t
			WHERE t.library_id IN ?
			AND LOWER(t.artist) = LOWER(c.artist)
			AND LOWER(t.title) = LOWER(c.title)
			AND LOWER(t.album) = LOWER(c.album)
			AND ABS(t.duration_ms - c.duration_ms) <= ?
		)`, args...).Scan(&ids).Error
	for _, id := range ids {
		has[id] = true
	}
	return has, err
}

func (r *TrackRepository) KnownSources(ctx context.Context, libraryIDs []uint, refs []provider.TrackRef) (map[provider.TrackRef]bool, error) {
	known := map[provider.TrackRef]bool{}
	if len(refs) == 0 {
		return known, nil
	}
	pairs := make([][]any, 0, len(refs))
	for _, ref := range refs {
		pairs = append(pairs, []any{ref.Provider, ref.ID})
	}
	var rows []library.TrackSource
	err := dbForContext(ctx, r.DB).
		Where("library_id IN ? AND (provider, ref) IN ?", libraryIDs, pairs).
		Find(&rows).Error
	for _, row := range rows {
		known[row.TrackRef()] = true
	}
	return known, err
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

func (r *TrackRepository) FindByMetadata(ctx context.Context, libraryIDs []uint, ms []library.Metadata) ([]*library.Track, error) {
	found := make([]*library.Track, len(ms))
	if len(ms) == 0 || len(libraryIDs) == 0 {
		return found, nil
	}
	values := make([]string, 0, len(ms))
	args := make([]any, 0, len(ms)*4+1)
	for n, m := range ms {
		values = append(values, "(?::int, ?::text, ?::text, ?::text)")
		args = append(args, n, m.Artist, m.Title, m.Album)
	}
	args = append(args, libraryIDs)

	var rows []struct {
		N       int
		TrackID uint
	}
	err := dbForContext(ctx, r.DB).Raw(`
		SELECT DISTINCT ON (c.n) c.n, t.id AS track_id
		FROM (VALUES `+strings.Join(values, ", ")+`) AS c(n, artist, title, album)
		JOIN tracks t ON t.library_id IN ?
			AND LOWER(t.artist) = LOWER(TRIM(c.artist))
			AND LOWER(t.title) = LOWER(TRIM(c.title))
		ORDER BY c.n, LOWER(t.album) = LOWER(TRIM(c.album)) DESC, t.id`, args...).Scan(&rows).Error
	if err != nil || len(rows) == 0 {
		return found, err
	}

	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.TrackID)
	}
	loaded, err := r.GetMany(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[uint]*library.Track, len(loaded))
	for n := range loaded {
		byID[loaded[n].ID] = &loaded[n]
	}
	for _, row := range rows {
		found[row.N] = byID[row.TrackID]
	}
	return found, nil
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

func (r *TrackRepository) SourceSongs(
	ctx context.Context,
	libraryIDs []uint,
	providerName provider.ProviderName,
	refs []string,
) (map[string]string, error) {
	var rows []struct {
		Ref    string
		SongID string
	}
	err := dbForContext(ctx, r.DB).
		Model(&library.TrackSource{}).
		Select("track_sources.ref, tracks.song_id").
		Joins("JOIN tracks ON tracks.id = track_sources.track_id").
		Where("track_sources.library_id IN ? AND track_sources.provider = ? AND track_sources.ref IN ?", libraryIDs, providerName, refs).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	songs := make(map[string]string, len(rows))
	for _, row := range rows {
		songs[row.Ref] = row.SongID
	}
	return songs, nil
}

func (r *TrackRepository) InLibrary(ctx context.Context, libraryID uint) ([]library.Track, error) {
	var all []library.Track
	err := dbForContext(ctx, r.DB).Where("library_id = ?", libraryID).Order("id").Find(&all).Error
	return all, err
}

func (r *TrackRepository) BySongs(ctx context.Context, libraryIDs []uint, songIDs []string) (map[string]*library.Track, error) {
	var found []library.Track
	err := tracks(ctx, r.DB).Where("library_id IN ? AND song_id IN ?", libraryIDs, songIDs).Find(&found).Error
	bySong := make(map[string]*library.Track, len(found))
	for n := range found {
		bySong[found[n].SongID] = &found[n]
	}
	return bySong, err
}

func (r *TrackRepository) CountIn(ctx context.Context, libraryIDs []uint) (int64, error) {
	var count int64
	err := dbForContext(ctx, r.DB).Model(&library.Track{}).Where("library_id IN ?", libraryIDs).Count(&count).Error
	return count, err
}

func (r *TrackRepository) Weigh(ctx context.Context, libraryIDs []uint) (map[uint]int64, error) {
	var rows []struct {
		LibraryID uint
		Size      int64
	}
	err := dbForContext(ctx, r.DB).
		Model(&library.Track{}).
		Select("library_id, SUM(size) AS size").
		Where("library_id IN ?", libraryIDs).
		Group("library_id").
		Scan(&rows).Error
	weights := make(map[uint]int64, len(rows))
	for _, row := range rows {
		weights[row.LibraryID] = row.Size
	}
	return weights, err
}

func (r *TrackRepository) Delete(ctx context.Context, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	return dbForContext(ctx, r.DB).Delete(&library.Track{}, ids).Error
}

// tracks loads Tracks whole, with their Sources.
func tracks(ctx context.Context, db *gorm.DB) *gorm.DB {
	return dbForContext(ctx, db).Preload("Sources", func(q *gorm.DB) *gorm.DB { return q.Order("id") })
}
