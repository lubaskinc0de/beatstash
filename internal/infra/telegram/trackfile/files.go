// Package trackfile keeps the Telegram files of Tracks: inline mode and
// the feed send a Track by its file instead of uploading it again.
package trackfile

import (
	"context"
	"maps"
	"slices"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
)

type FileKind string

const (
	FileAudio    FileKind = "audio"
	FileDocument FileKind = "document"
)

type File struct {
	TrackID  uint     `gorm:"primaryKey;autoIncrement:false"`
	ID       string   `gorm:"column:file_id;not null"`
	UniqueID string   `gorm:"column:file_unique_id;not null;index"`
	Kind     FileKind `gorm:"not null"`
}

func (File) TableName() string {
	return "telegram_files"
}

type Files struct {
	DB     *gorm.DB
	Sender *AudioSender
	// LeaseTTL outlasts a post: an older claim on one was left by an
	// instance that died amid it.
	LeaseTTL time.Duration
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
	err := f.DB.WithContext(ctx).
		Table("tracks").
		Select("DISTINCT ON (tracks.id) tracks.id AS for_track, telegram_files.*").
		Joins("JOIN telegram_files ON telegram_files.track_id = tracks.id OR telegram_files.track_id IN (?)", f.copies()).
		Where("tracks.id IN ?", trackIDs).
		Order("tracks.id").
		Order("telegram_files.track_id = tracks.id DESC").
		Order(clause.Expr{SQL: "telegram_files.kind = ? DESC", Vars: []any{FileAudio}, WithoutParentheses: true}).
		Order("telegram_files.track_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	files := make(map[uint]*File, len(rows))
	for n := range rows {
		files[rows[n].ForTrack] = &rows[n].File
	}
	return files, nil
}

// copies lists the Tracks sharing a Track Ref with tracks.id of the outer
// query: copies made by Share and Take, and the same file sent by several
// users. They have the same audio, so one file serves them all.
func (f *Files) copies() *gorm.DB {
	return f.DB.
		Table("track_sources source").
		Select("copies.track_id").
		Joins("JOIN track_sources copies ON copies.provider = source.provider AND copies.ref = source.ref").
		Where("source.track_id = tracks.id")
}

// Remember keeps a file sendable as audio over a document. A Track deleted
// meanwhile gets no file.
func (f *Files) Remember(ctx context.Context, files ...File) error {
	kept := make(map[uint]File, len(files))
	for _, file := range files {
		if _, ok := kept[file.TrackID]; !ok || file.Kind == FileAudio {
			kept[file.TrackID] = file
		}
	}
	var existing []uint
	err := f.DB.WithContext(ctx).Model(&library.Track{}).Where("id IN ?", slices.Collect(maps.Keys(kept))).Pluck("id", &existing).Error
	if err != nil || len(existing) == 0 {
		return err
	}
	rows := make([]File, 0, len(existing))
	for _, id := range existing {
		rows = append(rows, kept[id])
	}
	return f.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "track_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"file_id", "file_unique_id", "kind"}),
		Where: clause.Where{Exprs: []clause.Expression{
			clause.Expr{SQL: "telegram_files.kind <> ? AND excluded.kind = ?", Vars: []any{FileAudio, FileAudio}},
		}},
	}).Create(&rows).Error
}

// Recognize lists the Tracks whose file has the unique id, with their copies.
func (f *Files) Recognize(ctx context.Context, uniqueID string) ([]uint, error) {
	withFile := f.DB.Table("telegram_files").Select("track_id").Where("file_unique_id = ?", uniqueID)
	copies := f.DB.
		Table("telegram_files file").
		Select("copies.track_id").
		Joins("JOIN track_sources source ON source.track_id = file.track_id").
		Joins("JOIN track_sources copies ON copies.provider = source.provider AND copies.ref = source.ref").
		Where("file.file_unique_id = ?", uniqueID)
	var ids []uint
	err := f.DB.WithContext(ctx).Table("(?) AS recognized", f.DB.Raw("? UNION ?", withFile, copies)).Pluck("track_id", &ids).Error
	return ids, err
}

type Unfiled struct {
	library.Track
	// Dir is its Library's, relative to music_dir.
	Dir string
}

// Unfiled lists Tracks without a file of their own or of a copy, except
// those that came from Telegram: their file is the one the user sent. It
// skips the Tracks being posted now and those Telegram would not take
// under the current size limit.
func (f *Files) Unfiled(ctx context.Context, limit int) ([]Unfiled, error) {
	var tracks []Unfiled
	err := f.DB.WithContext(ctx).
		Table("tracks").
		Select("tracks.*, libraries.dir").
		Joins("JOIN libraries ON libraries.id = tracks.library_id").
		Where(`NOT EXISTS (
			SELECT 1 FROM telegram_files
			WHERE telegram_files.track_id = tracks.id OR telegram_files.track_id IN (?)
		)`, f.copies()).
		Where("NOT EXISTS (SELECT 1 FROM track_sources WHERE track_sources.track_id = tracks.id AND provider = ?)", provider.ProviderTelegram).
		Where(`NOT EXISTS (
			SELECT 1 FROM telegram_file_posts posts
			WHERE posts.track_id = tracks.id AND (posts.claimed_until > now() OR posts.rejected_size > ?)
		)`, f.Sender.MaxPostSize).
		Order("tracks.id").Limit(limit).Scan(&tracks).Error
	return tracks, err
}
