package repositories

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
)

type Tracks interface {
	FindSource(ctx context.Context, libraryID uint, providerName provider.ProviderName, ref string) (*library.TrackSource, error)
	// FindDuplicate finds a Track that is a Duplicate of the audio: artist,
	// title and album match ignoring case, and durations differ by at most
	// library.DuplicateToleranceMs. Audio without artist or title has no
	// Duplicates.
	FindDuplicate(ctx context.Context, libraryID uint, m library.Metadata, durationMs int) (*library.Track, error)
	// FindByMetadata matches artist and title in any of the libraries,
	// preferring the same album.
	FindByMetadata(ctx context.Context, libraryIDs []uint, m library.Metadata) (*library.Track, error)
	// SaveTrack saves the Track's new Sources along with it.
	SaveTrack(ctx context.Context, track *library.Track) error
	Get(ctx context.Context, id uint) (*library.Track, error)
	// Album lists the library's Tracks of the album, by track number.
	Album(ctx context.Context, libraryID uint, albumArtist, album string) ([]library.Track, error)
	KnownRefs(ctx context.Context, libraryID uint, providerName provider.ProviderName, refs []string) ([]string, error)
	// SourcePaths maps the refs the library has a Track Source for to the
	// paths of their Tracks.
	SourcePaths(ctx context.Context, libraryID uint, providerName provider.ProviderName, refs []string) (map[string]string, error)
}

var (
	ErrTrackNotFound  = errors.New("track not found")
	ErrSourceNotFound = errors.New("track source not found")
)
