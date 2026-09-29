package library

import (
	"slices"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
)

// Track is an aggregate root. Its TrackSources are part of it.
type Track struct {
	ID uint `gorm:"primaryKey"`

	LibraryID uint `gorm:"not null;uniqueIndex:idx_track_library_path"`
	// Path is relative to the Library's Dir.
	Path string `gorm:"not null;uniqueIndex:idx_track_library_path"`

	Metadata
	Quality

	DurationMs int
	Format     Format

	// SongID is the Navidrome song a Track of an Attached Library follows;
	// empty for other Tracks.
	SongID string `gorm:"index"`

	// Sources: one per Track Ref.
	Sources []*TrackSource `gorm:"constraint:OnDelete:CASCADE;"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

// TrackSource is an entity inside Track: a Track Ref this Track was received
// through.
type TrackSource struct {
	ID uint `gorm:"primaryKey"`

	TrackID uint `gorm:"not null;index"`

	// LibraryID is the Track's LibraryID, copied here for the unique index:
	// one Track per Track Ref in a Library.
	LibraryID uint                  `gorm:"not null;uniqueIndex:idx_track_source_library_ref"`
	Provider  provider.ProviderName `gorm:"not null;uniqueIndex:idx_track_source_library_ref"`
	Ref       string                `gorm:"not null;uniqueIndex:idx_track_source_library_ref"`

	CreatedAt time.Time
}

// DuplicateToleranceMs: files of the same recording from different sources
// can differ in duration by rounding. A bigger difference means a different
// version.
const DuplicateToleranceMs = 2000

// NewTrack creates a Track at its LayoutPath. Without artist or title the
// Track goes to the Inbox.
func NewTrack(lib *Library, in Incoming) (*Track, Outcome) {
	t := &Track{
		LibraryID:  lib.ID,
		Path:       LayoutPath(in.Metadata, in.Format, in.OriginalName),
		Metadata:   in.Metadata,
		Quality:    in.Quality,
		DurationMs: in.DurationMs,
		Format:     in.Format,
	}
	t.AddSource(in.Ref)
	if !in.Metadata.Complete() {
		return t, StoredInInbox
	}
	return t, Stored
}

// NewAttachedTrack follows a song of an Attached Library. It has no Layout
// and no Sources yet.
func NewAttachedTrack(lib *Library, song Song) *Track {
	t := &Track{LibraryID: lib.ID}
	t.Follow(song)
	return t
}

// Follow takes the song as Navidrome has it now, with the tags' spaces
// tidied so Duplicates match; changed is false if nothing differs.
func (t *Track) Follow(song Song) (changed bool) {
	before := *t
	t.SongID = song.ID
	t.Path = song.Path
	t.Metadata = song.Metadata.Normalize()
	t.DurationMs = song.DurationMs
	t.Format = song.Format
	t.Quality = song.Quality
	return before.SongID != t.SongID || before.Path != t.Path || before.Metadata != t.Metadata ||
		before.DurationMs != t.DurationMs || before.Format != t.Format || before.Quality != t.Quality
}

// Attached reports whether the Track is in an Attached Library: the bot
// never replaces, moves, retags or deletes its file.
func (t *Track) Attached() bool {
	return t.SongID != ""
}

// Absorb handles incoming audio that is a Duplicate of the Track. The
// Source is always added. The file is replaced only if the new Quality is
// better and the Track is not Attached. The metadata stays, so only the
// path's extension can change.
func (t *Track) Absorb(in Incoming) Outcome {
	t.AddSource(in.Ref)
	if t.Attached() || !in.Quality.Better(t.Quality) {
		return AlreadyExists
	}
	t.Path = LayoutPath(t.Metadata, in.Format, in.OriginalName)
	t.Format = in.Format
	t.Quality = in.Quality
	t.DurationMs = in.DurationMs
	return Replaced
}

// MoveTo sets a new path, for example when the chosen one is taken; an
// Attached Track keeps its own.
func (t *Track) MoveTo(path string) {
	if t.Attached() {
		return
	}
	t.Path = path
}

func (t *Track) AddSource(ref provider.TrackRef) *TrackSource {
	for _, s := range t.Sources {
		if s.Provider == ref.Provider && s.Ref == ref.ID {
			return s
		}
	}
	s := &TrackSource{TrackID: t.ID, LibraryID: t.LibraryID, Provider: ref.Provider, Ref: ref.ID}
	t.Sources = append(t.Sources, s)
	return s
}

func (t *Track) In(library *Library) bool {
	return t.LibraryID == library.ID
}

// KeptIn returns ErrNotKeptTrack unless one of the user's Kept Libraries
// has the Track.
func (t *Track) KeptIn(kept []*Library) error {
	if !slices.ContainsFunc(kept, t.In) {
		return ErrNotKeptTrack
	}
	return nil
}

// AudibleBy returns ErrNotKeptTrack unless the user can listen to the
// Track: one of their Kept Libraries or the Shared Library has it.
func (t *Track) AudibleBy(kept []*Library, shared *Library) error {
	if t.In(shared) {
		return nil
	}
	return t.KeptIn(kept)
}

// ShareableBy reports why a user cannot share the Track: it is not theirs,
// or it is in the Inbox (without artist or title nobody would find it).
func (t *Track) ShareableBy(kept []*Library) error {
	if err := t.KeptIn(kept); err != nil {
		return err
	}
	if !t.Complete() {
		return ErrInboxTrack
	}
	return nil
}

func (t *Track) Single() bool {
	return t.Album == ""
}

// CopyTo copies the Track without Sources: a Track Ref can be used once
// per Library, and only the caller can check which ones the target Library
// already has. The copy is the bot's own and follows no song.
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

func (s *TrackSource) TrackRef() provider.TrackRef {
	return provider.TrackRef{Provider: s.Provider, ID: s.Ref}
}
