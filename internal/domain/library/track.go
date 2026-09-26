package library

import (
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

// Absorb handles incoming audio that is a Duplicate of the Track. The
// Source is always added. The file is replaced only if the new Quality is
// better. The metadata stays, so only the path's extension can change.
func (t *Track) Absorb(in Incoming) Outcome {
	t.AddSource(in.Ref)
	if !in.Quality.Better(t.Quality) {
		return AlreadyExists
	}
	t.Path = LayoutPath(t.Metadata, in.Format, in.OriginalName)
	t.Format = in.Format
	t.Quality = in.Quality
	t.DurationMs = in.DurationMs
	return Replaced
}

// MoveTo sets a new path, for example when the chosen one is taken.
func (t *Track) MoveTo(path string) {
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

// OwnedBy returns ErrNotOwnTrack unless the Track is in the personal Library.
func (t *Track) OwnedBy(personal *Library) error {
	if !t.In(personal) {
		return ErrNotOwnTrack
	}
	return nil
}

// ShareableBy reports why the owner of personal cannot share the Track: it
// is not theirs, or it is in the Inbox (without artist or title nobody would
// find it).
func (t *Track) ShareableBy(personal *Library) error {
	if err := t.OwnedBy(personal); err != nil {
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

// CopyTo copies the Track without Sources. A Track Ref can be used once
// per Library, and only the caller can check which ones the target Library
// already has.
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
