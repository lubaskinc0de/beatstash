package navidrome

import (
	"context"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

type Credentials struct {
	Login    string
	Password string
}

type Client interface {
	Authenticate(ctx context.Context, creds Credentials) error
	NowPlaying(ctx context.Context, creds Credentials) (*PlayingTrack, error)
	RecentlyPlayed(ctx context.Context, creds Credentials, limit int) ([]PlayedTrack, error)
	// CreateAccount returns ErrLoginTaken if the login is in use, whatever its case.
	CreateAccount(ctx context.Context, admin, account Credentials) error

	Libraries(ctx context.Context, admin Credentials) ([]Library, error)
	// CreateLibrary returns ErrNameTaken if another library has the name.
	CreateLibrary(ctx context.Context, admin Credentials, lib Library) (int, error)
	UpdateLibrary(ctx context.Context, admin Credentials, lib Library) error
	// SetLibraries replaces the libraries the account may see.
	SetLibraries(ctx context.Context, admin Credentials, login string, libraryIDs []int) error

	// Songs maps paths relative to the library to song ids, for the songs
	// Navidrome has indexed.
	Songs(ctx context.Context, creds Credentials, libraryID int) (map[string]string, error)
	Star(ctx context.Context, creds Credentials, songIDs []string) error
	Unstar(ctx context.Context, creds Credentials, songIDs []string) error
	// SavePlaylist replaces the songs of the playlist, or creates it when
	// id is empty or the playlist is gone, and returns its id.
	SavePlaylist(ctx context.Context, creds Credentials, id, name string, songIDs []string) (string, error)
}

type Library struct {
	ID   int
	Name string
	Path string
	// DefaultNewUsers: Navidrome gives the library to every account it creates.
	DefaultNewUsers bool
}

type Track struct {
	ID       string
	Artist   string
	Title    string
	Album    string
	Duration int
}

func (t Track) Metadata() library.Metadata {
	return library.Metadata{Artist: t.Artist, Title: t.Title, Album: t.Album}
}

type PlayingTrack struct {
	Track
	PositionMs int
	State      string
	CoverArt   string
}

type PlayedTrack struct {
	Track
	PlayedAt time.Time
}
