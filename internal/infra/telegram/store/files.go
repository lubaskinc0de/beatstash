package store

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

type FileKind string

const (
	FileAudio    FileKind = "audio"
	FileDocument FileKind = "document"
)

// File is a Telegram file with a Track's audio: inline mode and the feed
// send it by file_id instead of uploading the Track again.
type File struct {
	TrackID  uint     `gorm:"primaryKey;autoIncrement:false"`
	ID       string   `gorm:"column:file_id;not null"`
	UniqueID string   `gorm:"column:file_unique_id;not null;index"`
	Kind     FileKind `gorm:"not null"`
}

func (File) TableName() string {
	return "telegram_files"
}

// Upload claims a Track's upload to Telegram: the claimant uploads, the
// others wait for the file instead of uploading it too.
type Upload struct {
	TrackID   uint      `gorm:"primaryKey;autoIncrement:false"`
	StartedAt time.Time `gorm:"not null"`
}

func (Upload) TableName() string {
	return "telegram_uploads"
}

// uploadTTL outlasts any upload, bounded by the Bot API client's timeout:
// an older claim was left by a bot that stopped mid-upload.
const uploadTTL = "5 minutes"

// copiesOf lists the Tracks sharing a Track Ref with @track: copies made by
// Share and Take, and the same file sent by several users. They have the
// same audio, so one file serves them all.
const copiesOf = `
	SELECT copies.track_id FROM track_sources source
	JOIN track_sources copies ON copies.provider = source.provider AND copies.ref = source.ref
	WHERE source.track_id = @track`

type Files struct {
	DB *gorm.DB
}

// Of maps each of the Tracks that has a file, of its own or of a copy, to
// it; it prefers the Track's own file, then one Telegram plays as audio.
func (f *Files) Of(ctx context.Context, trackIDs []uint) (map[uint]*File, error) {
	if len(trackIDs) == 0 {
		return map[uint]*File{}, nil
	}
	var rows []struct {
		ForTrack uint
		File
	}
	err := f.DB.WithContext(ctx).Raw(`
		SELECT DISTINCT ON (tracks.id) tracks.id AS for_track, telegram_files.*
		FROM tracks JOIN telegram_files
		ON telegram_files.track_id = tracks.id
		OR telegram_files.track_id IN (`+strings.ReplaceAll(copiesOf, "@track", "tracks.id")+`)
		WHERE tracks.id IN @tracks
		ORDER BY tracks.id, telegram_files.track_id = tracks.id DESC, telegram_files.kind = @audio DESC, telegram_files.track_id`,
		sql.Named("tracks", trackIDs), sql.Named("audio", FileAudio),
	).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	files := make(map[uint]*File, len(rows))
	for n := range rows {
		files[rows[n].ForTrack] = &rows[n].File
	}
	return files, nil
}

// Remember keeps a file sendable as audio over a document.
func (f *Files) Remember(ctx context.Context, file File) error {
	return f.DB.WithContext(ctx).Exec(`
		INSERT INTO telegram_files (track_id, file_id, file_unique_id, kind) VALUES (?, ?, ?, ?)
		ON CONFLICT (track_id) DO UPDATE
		SET file_id = EXCLUDED.file_id, file_unique_id = EXCLUDED.file_unique_id, kind = EXCLUDED.kind
		WHERE telegram_files.kind <> ? AND EXCLUDED.kind = ?`,
		file.TrackID, file.ID, file.UniqueID, file.Kind, FileAudio, FileAudio,
	).Error
}

// Claim tells whether the caller may upload the Track: nobody else is at it,
// or their claim is abandoned.
func (f *Files) Claim(ctx context.Context, trackID uint) (bool, error) {
	result := f.DB.WithContext(ctx).Exec(`
		INSERT INTO telegram_uploads (track_id, started_at) VALUES (?, now())
		ON CONFLICT (track_id) DO UPDATE SET started_at = now()
		WHERE telegram_uploads.started_at < now() - interval '`+uploadTTL+`'`,
		trackID,
	)
	return result.RowsAffected == 1, result.Error
}

// Release comes after Remember: a waiter that finds the claim gone finds
// the file.
func (f *Files) Release(ctx context.Context, trackID uint) error {
	return f.DB.WithContext(ctx).Exec("DELETE FROM telegram_uploads WHERE track_id = ?", trackID).Error
}

// Recognize lists the Tracks whose file has the unique id, with their copies.
func (f *Files) Recognize(ctx context.Context, uniqueID string) ([]uint, error) {
	var ids []uint
	err := f.DB.WithContext(ctx).Raw(`
		SELECT track_id FROM telegram_files WHERE file_unique_id = @unique
		UNION
		SELECT copies.track_id FROM telegram_files file
		JOIN track_sources source ON source.track_id = file.track_id
		JOIN track_sources copies ON copies.provider = source.provider AND copies.ref = source.ref
		WHERE file.file_unique_id = @unique`,
		sql.Named("unique", uniqueID),
	).Scan(&ids).Error
	return ids, err
}

type Unfiled struct {
	library.Track
	// Dir is its Library's, relative to music_dir.
	Dir string
}

// Unfiled lists Tracks without a file of their own or of a copy, except
// those that came from Telegram: their file is the one the user sent.
func (f *Files) Unfiled(ctx context.Context, except []uint, limit int) ([]Unfiled, error) {
	query := f.DB.WithContext(ctx).
		Table("tracks").
		Select("tracks.*, libraries.dir").
		Joins("JOIN libraries ON libraries.id = tracks.library_id").
		Where(`NOT EXISTS (
			SELECT 1 FROM telegram_files
			WHERE telegram_files.track_id = tracks.id
			OR telegram_files.track_id IN (`+strings.ReplaceAll(copiesOf, "@track", "tracks.id")+`)
		)`).
		Where("NOT EXISTS (SELECT 1 FROM track_sources WHERE track_sources.track_id = tracks.id AND provider = ?)", "telegram")
	if len(except) > 0 {
		query = query.Where("tracks.id NOT IN ?", except)
	}
	var tracks []Unfiled
	err := query.Order("tracks.id").Limit(limit).Scan(&tracks).Error
	return tracks, err
}
