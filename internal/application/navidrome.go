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
)

type NavidromeCredentials struct {
	Login    string
	Password string
}

type Navidrome interface {
	// Authenticate returns ErrNavidromeInvalidCredentials for a wrong login or password.
	Authenticate(ctx context.Context, creds NavidromeCredentials) error
	// NowPlaying returns what the account is playing, or nil.
	NowPlaying(ctx context.Context, creds NavidromeCredentials) (*PlayingTrack, error)
	RecentlyPlayed(ctx context.Context, creds NavidromeCredentials, limit int) ([]PlayedTrack, error)
	// CreateAccount returns ErrNavidromeLoginTaken if the login is in use, whatever its case.
	CreateAccount(ctx context.Context, admin, account NavidromeCredentials) error
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
