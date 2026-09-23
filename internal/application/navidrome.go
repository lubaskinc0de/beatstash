package application

import (
	"context"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type Navidrome interface {
	// NowPlaying returns what the bot's account is playing, or nil.
	NowPlaying(ctx context.Context) (*PlayingTrack, error)
	RecentlyPlayed(ctx context.Context, limit int) ([]PlayedTrack, error)
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
