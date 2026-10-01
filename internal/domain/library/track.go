// Track: one audio file in a Library with its metadata, Quality and Track
// Sources. Path is relative to the Library's Dir; Size is in bytes.
// FileVersion grows each time the content of the file changes, and a move
// keeps it. FileModTime and FileInode are the file as the bot last saw it,
// zero until it records them. SongID is set only on a Track of an Attached
// Library. A TrackSource copies its Track's LibraryID for the unique index:
// one Track per Track Ref in a Library.

package library

import (
	"slices"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
)

// Track is an aggregate root. Its TrackSources are part of it.
type Track struct {
	ID uint `gorm:"primaryKey"`

	LibraryID uint   `gorm:"not null;uniqueIndex:idx_track_library_path"`
	Path      string `gorm:"not null;uniqueIndex:idx_track_library_path"`

	Metadata
	Quality

	DurationMs  int
	Format      Format
	Size        int64     `gorm:"not null;default:0"`
	FileVersion int       `gorm:"not null;default:0"`
	FileModTime time.Time `gorm:"not null;default:'0001-01-01 00:00:00+00'"`
	FileInode   uint64    `gorm:"not null;default:0"`

	SongID string `gorm:"index"`

	Sources []*TrackSource `gorm:"constraint:OnDelete:CASCADE;"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

// TrackSource is an entity inside Track: a Track Ref this Track was received
// through.
type TrackSource struct {
	ID uint `gorm:"primaryKey"`

	TrackID uint `gorm:"not null;index"`

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

// NewFoundTrack is a Track for a file put into a Library by hand: it has no
// Sources and keeps the file's path. Without artist or title it counts as
// in the Inbox.
func NewFoundTrack(lib *Library, file LibraryFile, probe Probe) *Track {
	t := &Track{LibraryID: lib.ID, Path: file.Path}
	t.takeAudio(file, probe)
	return t
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
	t.Size = song.Size
	return before.SongID != t.SongID || before.Path != t.Path || before.Metadata != t.Metadata ||
		before.DurationMs != t.DurationMs || before.Format != t.Format || before.Quality != t.Quality ||
		before.Size != t.Size
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
	t.FileVersion++
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

// GrowthTo is how much the Library grows once the Track's file has the
// size: a new Track by all of it, a replaced one by the difference.
func (t *Track) GrowthTo(size int64) int64 {
	return size - t.Size
}

// RecordFile keeps what the Track's file is like now, before it goes to
// the Track's path: a rename keeps all of it.
func (t *Track) RecordFile(file LibraryFile) {
	t.Size = file.Size
	t.FileModTime = file.ModTime
	t.FileInode = file.Inode
}

// Holds tells whether the file at the Track's path is the one the Track
// recorded.
func (t *Track) Holds(file LibraryFile) bool {
	return file.holds(t.Size, t.FileModTime)
}

// FollowFile takes the Track's audio from its file changed by hand: the
// metadata from its tags, with spaces tidied so Duplicates match, and the
// rest from the audio itself. The path stays. A Track that never recorded
// its file cannot tell whether the audio changed, so its FileVersion stays.
func (t *Track) FollowFile(file LibraryFile, probe Probe) {
	recorded := t.recorded()
	t.takeAudio(file, probe)
	if recorded {
		t.FileVersion++
	}
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
// already has. The copy is the bot's own and follows no song. Its file is
// a hardlink of the Track's.
func (t *Track) CopyTo(library *Library, path string) *Track {
	return &Track{
		LibraryID:   library.ID,
		Path:        path,
		Metadata:    t.Metadata,
		Quality:     t.Quality,
		DurationMs:  t.DurationMs,
		Format:      t.Format,
		Size:        t.Size,
		FileModTime: t.FileModTime,
		FileInode:   t.FileInode,
	}
}

func (s *TrackSource) TrackRef() provider.TrackRef {
	return provider.TrackRef{Provider: s.Provider, ID: s.Ref}
}

func (t *Track) takeAudio(file LibraryFile, probe Probe) {
	if format, ok := file.Format(); ok {
		t.Format = format
	}
	t.Metadata = probe.Tags.Normalize()
	t.Quality = probe.Quality
	t.DurationMs = probe.DurationMs
	t.RecordFile(file)
}

// recorded is false for a Track that never recorded its file.
func (t *Track) recorded() bool {
	return !t.FileModTime.IsZero()
}
