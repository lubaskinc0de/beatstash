package repositories

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type Tracks interface {
	FindSource(ctx context.Context, libraryID uint, provider domain.ProviderName, ref string) (*domain.TrackSource, error)
	FindDuplicate(ctx context.Context, libraryID uint, m domain.Metadata, durationMs int) (*domain.Track, error)
	// FindByMetadata matches artist and title, preferring the same album.
	FindByMetadata(ctx context.Context, libraryID uint, m domain.Metadata) (*domain.Track, error)
	SaveTrack(ctx context.Context, track *domain.Track) error
	SaveSource(ctx context.Context, source *domain.TrackSource) error
	FindTelegramFile(ctx context.Context, libraryIDs []uint, m domain.Metadata) (*domain.TelegramFile, error)
	Get(ctx context.Context, id uint) (*domain.Track, error)
	Sources(ctx context.Context, trackID uint) ([]domain.TrackSource, error)
	// Album lists the library's Tracks of the album, by track number.
	Album(ctx context.Context, libraryID uint, albumArtist, album string) ([]domain.Track, error)
	Delete(ctx context.Context, id uint) error
	// TelegramFile prefers files sendable as audio; ErrNoTelegramFile if the Track has none.
	TelegramFile(ctx context.Context, trackID uint) (*domain.TelegramFile, error)
	// Copies lists the Tracks of any library that share a Track Ref of
	// another Provider with the Track, the Track included.
	Copies(ctx context.Context, trackID uint) ([]domain.Track, error)
	KnownRefs(ctx context.Context, libraryID uint, provider domain.ProviderName, refs []string) ([]string, error)
	// SourcePaths maps the refs the library has a Track Source for to the
	// paths of their Tracks.
	SourcePaths(ctx context.Context, libraryID uint, provider domain.ProviderName, refs []string) (map[string]string, error)
}

var (
	ErrTrackNotFound  = errors.New("track not found")
	ErrSourceNotFound = errors.New("track source not found")
	ErrNoTelegramFile = errors.New("track has no telegram file")
)
