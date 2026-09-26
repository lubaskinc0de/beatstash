package library

import (
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
)

type Track struct {
	ID uint `gorm:"primaryKey"`

	LibraryID uint `gorm:"not null;uniqueIndex:idx_track_library_path"`
	// Path is relative to the Library's Dir.
	Path string `gorm:"not null;uniqueIndex:idx_track_library_path"`

	Metadata
	Quality

	DurationMs int
	Format     Format

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (t *Track) In(library *Library) bool {
	return t.LibraryID == library.ID
}

// Shareable refuses Inbox Tracks: without artist or title nobody finds them.
func (t *Track) Shareable() error {
	if !t.Complete() {
		return ErrInboxTrack
	}
	return nil
}

func (t *Track) Single() bool {
	return t.Album == ""
}

func ShareableTracks(album []Track) []Track {
	var shareable []Track
	for _, t := range album {
		if t.Shareable() == nil {
			shareable = append(shareable, t)
		}
	}
	return shareable
}

// CopyTo leaves the Track Refs to TrackSource.For.
func (t *Track) CopyTo(library *Library, path string) *Track {
	return &Track{
		LibraryID:  library.ID,
		Path:       path,
		Metadata:   t.Metadata,
		Quality:    t.Quality,
		DurationMs: t.DurationMs,
		Format:     t.Format,
	}
}

// DuplicateToleranceMs: tags of the same recording from different sources
// may round its length differently; a bigger gap means another edit.
const DuplicateToleranceMs = 2000

type Quality struct {
	Lossless bool
	Bitrate  int
}

// Better tells whether q beats other: lossless beats lossy,
// among lossy files the higher bitrate wins, ties keep other.
func (q Quality) Better(other Quality) bool {
	if q.Lossless != other.Lossless {
		return q.Lossless
	}
	if q.Lossless {
		return false
	}
	return q.Bitrate > other.Bitrate
}

type TrackSource struct {
	ID uint `gorm:"primaryKey"`

	TrackID uint  `gorm:"not null;index"`
	Track   Track `gorm:"constraint:OnDelete:CASCADE;"`

	// LibraryID repeats the Track's one: a Track Ref is unique per Library.
	LibraryID uint                  `gorm:"not null;uniqueIndex:idx_track_source_library_ref"`
	Provider  provider.ProviderName `gorm:"not null;uniqueIndex:idx_track_source_library_ref"`
	Ref       string                `gorm:"not null;uniqueIndex:idx_track_source_library_ref"`

	CreatedAt time.Time
}

func (s TrackSource) For(track *Track) *TrackSource {
	s.ID = 0
	s.TrackID = track.ID
	s.LibraryID = track.LibraryID
	return &s
}

type Upload struct {
	ID uint `gorm:"primaryKey"`

	UserID uint        `gorm:"not null;index"`
	User   access.User `gorm:"constraint:OnDelete:CASCADE;"`

	TrackID uint  `gorm:"not null;index"`
	Track   Track `gorm:"constraint:OnDelete:CASCADE;"`

	TrackSourceID uint        `gorm:"not null"`
	TrackSource   TrackSource `gorm:"constraint:OnDelete:CASCADE;"`

	CreatedAt time.Time
}
