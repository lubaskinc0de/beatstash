package application

import (
	"context"
	"log/slog"
)

type NowPlaying struct {
	ID         string
	Artist     string
	Title      string
	Album      string
	Duration   int
	PositionMs int
	State      string
	CoverArt   string
}

type GetNowPlaying struct {
	Client *NavidromeClient
}

func NewGetNowPlaying(client *NavidromeClient) *GetNowPlaying {
	return &GetNowPlaying{
		Client: client,
	}
}

func (i *GetNowPlaying) Execute(
	ctx context.Context,
) (*NowPlaying, error) {
	resp, err := i.Client.GetNowPlaying(ctx)
	if err != nil {
		slog.Error("Cannot get now playing", "error", err)
		return nil, err
	}

	entry := resp.SubsonicResponse.NowPlaying.Entry
	for _, track := range entry {
		if track.Username == i.Client.Username {
			return &NowPlaying{
				ID:         track.ID,
				Artist:     track.Artist,
				Title:      track.Title,
				Album:      track.Album,
				Duration:   track.Duration,
				PositionMs: track.PositionMs,
				CoverArt:   track.CoverArt,
				State:      track.State,
			}, nil
		}
	}
	return nil, nil
}
