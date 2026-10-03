package navidrome

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/lubaskinc0de/beatstash/internal/domain/library"
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
	// Account returns ErrAccountNotFound if Navidrome has no such login.
	Account(ctx context.Context, admin Credentials, login string) (*Account, error)
	// SetLibraries replaces the libraries the account may see.
	SetLibraries(ctx context.Context, admin Credentials, accountID string, libraryIDs []int) error

	// Songs maps paths relative to the library to song ids, for the songs
	// Navidrome has indexed.
	Songs(ctx context.Context, creds Credentials, libraryID int) (map[string]string, error)
	// LibrarySongs lists the songs Navidrome has indexed in the library and
	// whose files are still there.
	LibrarySongs(ctx context.Context, admin Credentials, libraryID int) ([]Song, error)
	// SongAt finds the song Navidrome has indexed at the path relative to
	// the library; empty if none.
	SongAt(ctx context.Context, admin Credentials, libraryID int, path string) (string, error)
	AlbumOf(ctx context.Context, creds Credentials, songID string) (string, error)
	// CreateShare makes a public link to the song or album and returns it,
	// at Navidrome's public address.
	CreateShare(ctx context.Context, creds Credentials, share Share) (string, error)
	// Download streams the song's file as it is; the caller closes it.
	Download(ctx context.Context, creds Credentials, songID string) (io.ReadCloser, error)
	Star(ctx context.Context, creds Credentials, songIDs []string) error
	Unstar(ctx context.Context, creds Credentials, songIDs []string) error
	// SavePlaylist replaces the songs of the playlist, or creates it when
	// id is empty or the playlist is gone, and returns its id.
	SavePlaylist(ctx context.Context, creds Credentials, id, name string, songIDs []string) (string, error)
}

type Share struct {
	// ID is a song's or an album's.
	ID           string
	Description  string
	Expires      time.Time
	Downloadable bool
}

type Library struct {
	ID   int
	Name string
	Path string
	// DefaultNewUsers: Navidrome gives the library to every account it creates.
	DefaultNewUsers bool
}

type Account struct {
	// ID is Navidrome's id of the account, not its login.
	ID     string
	Access library.NavidromeAccess
}

type Song struct {
	ID string
	// Path is relative to the library.
	Path        string
	AlbumArtist string
	Artist      string
	Album       string
	Title       string
	Year        int
	TrackNumber int
	DurationMs  int
	// Suffix is the file's extension without the dot.
	Suffix      string
	Codec       string
	BitrateKbps int
	// Size is the file's, in bytes.
	Size int64
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

const StatePaused = "paused"

func (t PlayingTrack) Paused() bool {
	return strings.EqualFold(t.State, StatePaused)
}

type PlayedTrack struct {
	Track
	PlayedAt time.Time
}
