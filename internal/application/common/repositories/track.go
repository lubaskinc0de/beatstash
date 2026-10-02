package repositories

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
)

type Tracks interface {
	// FindSource finds the Track Source of the ref in any of the libraries.
	FindSource(ctx context.Context, libraryIDs []uint, providerName provider.ProviderName, ref string) (*library.TrackSource, error)
	// FindDuplicate finds a Track in any of the libraries that is a
	// Duplicate of the audio: artist, title and album match ignoring case,
	// and durations differ by at most library.DuplicateToleranceMs. Audio
	// without artist or title has no Duplicates.
	FindDuplicate(ctx context.Context, libraryIDs []uint, m library.Metadata, durationMs int) (*library.Track, error)
	// FindByMetadata finds a Track for each metadata, nil where none: artist
	// and title match in any of the libraries, and the same album wins.
	FindByMetadata(ctx context.Context, libraryIDs []uint, ms []library.Metadata) ([]*library.Track, error)
	// SaveTrack saves the Track's new Sources along with it.
	SaveTrack(ctx context.Context, track *library.Track) error
	// Replace saves the Tracks and deletes the gone ones in a few statements.
	Replace(ctx context.Context, save, gone []*library.Track) error
	Get(ctx context.Context, id uint) (*library.Track, error)
	// GetMany leaves out the Tracks that are gone.
	GetMany(ctx context.Context, ids []uint) ([]library.Track, error)
	// WithDuplicates tells which of the tracks have a Duplicate in any of
	// the libraries.
	WithDuplicates(ctx context.Context, libraryIDs []uint, tracks []library.Track) (map[uint]bool, error)
	// KnownSources tells which of the refs any of the libraries has a Track
	// Source for.
	KnownSources(ctx context.Context, libraryIDs []uint, refs []provider.TrackRef) (map[provider.TrackRef]bool, error)
	// Album lists the library's Tracks of the album, by track number.
	Album(ctx context.Context, album library.AlbumKey) ([]library.Track, error)
	// SourcePaths maps the refs the library has a Track Source for to the
	// paths of their Tracks.
	SourcePaths(ctx context.Context, libraryID uint, providerName provider.ProviderName, refs []string) (map[string]string, error)
	// SourceSongs maps the refs the libraries have a Track Source for to the
	// Navidrome songs of their Tracks.
	SourceSongs(ctx context.Context, libraryIDs []uint, providerName provider.ProviderName, refs []string) (map[string]string, error)
	// InLibrary loads the Tracks without their Sources.
	InLibrary(ctx context.Context, libraryID uint) ([]library.Track, error)
	// InLibraries maps each of the libraries to its Tracks, without their
	// Sources.
	InLibraries(ctx context.Context, libraryIDs []uint) (map[uint][]library.Track, error)
	// AtPaths loads the library's Tracks at the paths, without their
	// Sources.
	AtPaths(ctx context.Context, libraryID uint, paths []string) ([]library.Track, error)
	// BySongs maps the Navidrome songs the libraries have Tracks of to them.
	BySongs(ctx context.Context, libraryIDs []uint, songIDs []string) (map[string]*library.Track, error)
	// Search lists the newest first for an empty text. The libraries go in
	// the order their copy of a Duplicate wins.
	Search(ctx context.Context, libraryIDs []uint, text string, offset, limit int) ([]library.Track, error)
	// SearchAlbums: indexedOnly leaves out the Albums Navidrome has indexed
	// none of the Tracks of.
	SearchAlbums(
		ctx context.Context, libraryIDs []uint, text string, indexedOnly bool, offset, limit int,
	) ([]AlbumSummary, error)
	// SetSongs keeps the Navidrome song of each Track, by its id; a Track
	// whose path has changed since keeps none.
	SetSongs(ctx context.Context, songs []library.Track) error
	Unindexed(ctx context.Context, libraryIDs []uint) (map[uint][]library.Track, error)
	CountIn(ctx context.Context, libraryIDs []uint) (int64, error)
	// Weigh maps each of the libraries to the sum of its Tracks' sizes.
	Weigh(ctx context.Context, libraryIDs []uint) (map[uint]int64, error)
}

// AlbumSummary stands for an Album by one of its Tracks, TrackID.
type AlbumSummary struct {
	library.AlbumKey
	TrackID uint
	Tracks  int
}

var (
	ErrTrackNotFound  = errors.New("track not found")
	ErrSourceNotFound = errors.New("track source not found")
)
