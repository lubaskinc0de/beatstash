package application

import (
	"context"
	"errors"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

var (
	ErrNavidromeInvalidCredentials = errors.New("navidrome: invalid credentials")
	ErrNavidromeLoginTaken         = errors.New("navidrome: login taken")
	ErrNavidromeNameTaken          = errors.New("navidrome: library name taken")
	// ErrNavidromeAdminAccount: Navidrome shows admins every library, so their access cannot be narrowed.
	ErrNavidromeAdminAccount = errors.New("navidrome: account is an admin")
)

type NavidromeCredentials struct {
	Login    string
	Password string
}

type Navidrome interface {
	Authenticate(ctx context.Context, creds NavidromeCredentials) error
	NowPlaying(ctx context.Context, creds NavidromeCredentials) (*PlayingTrack, error)
	RecentlyPlayed(ctx context.Context, creds NavidromeCredentials, limit int) ([]PlayedTrack, error)
	// CreateAccount returns ErrNavidromeLoginTaken if the login is in use, whatever its case.
	CreateAccount(ctx context.Context, admin, account NavidromeCredentials) error

	Libraries(ctx context.Context, admin NavidromeCredentials) ([]NavidromeLibrary, error)
	// CreateLibrary returns ErrNavidromeNameTaken if another library has the name.
	CreateLibrary(ctx context.Context, admin NavidromeCredentials, library NavidromeLibrary) (int, error)
	UpdateLibrary(ctx context.Context, admin NavidromeCredentials, library NavidromeLibrary) error
	// SetLibraries replaces the libraries the account may see.
	SetLibraries(ctx context.Context, admin NavidromeCredentials, login string, libraryIDs []int) error
}

type NavidromeLibrary struct {
	ID   int
	Name string
	Path string
	// DefaultNewUsers: Navidrome gives the library to every account it creates.
	DefaultNewUsers bool
}

type NavidromeTrack struct {
	ID       string
	Artist   string
	Title    string
	Album    string
	Duration int
}

func (t NavidromeTrack) Metadata() domain.Metadata {
	return domain.Metadata{Artist: t.Artist, Title: t.Title, Album: t.Album}
}

type PlayingTrack struct {
	NavidromeTrack
	PositionMs int
	State      string
	CoverArt   string
}

type PlayedTrack struct {
	NavidromeTrack
	PlayedAt time.Time
}
