package application

import (
	"context"
	"errors"
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

	TelegramFileID string
}

type GetNowPlaying struct {
	Client *NavidromeClient
	Repo   TrackRepository
}

func NewGetNowPlaying(client *NavidromeClient, repo TrackRepository) *GetNowPlaying {
	return &GetNowPlaying{
		Client: client,
		Repo:   repo,
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
		if track.Username != i.Client.Username {
			continue
		}

		nowPlaying := &NowPlaying{
			ID:         track.ID,
			Artist:     track.Artist,
			Title:      track.Title,
			Album:      track.Album,
			Duration:   track.Duration,
			PositionMs: track.PositionMs,
			CoverArt:   track.CoverArt,
			State:      track.State,
		}

		saved, err := i.Repo.FindByTitleAndPerformer(ctx, track.Title, track.Artist)
		switch {
		case err == nil:
			nowPlaying.TelegramFileID = saved.TelegramFileID
		case errors.Is(err, ErrTrackNotFound):
			slog.Info("now_playing_track_not_in_db", "title", track.Title, "artist", track.Artist)
		default:
			slog.Error("find_track", "error", err)
		}

		return nowPlaying, nil
	}
	return nil, nil
}
