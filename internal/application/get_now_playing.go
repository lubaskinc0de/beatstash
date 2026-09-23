package application

import (
	"context"
	"errors"
	"log/slog"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type NowPlaying struct {
	PlayingTrack
	TelegramFile *domain.TelegramFile
}

type GetNowPlaying struct {
	Client Navidrome
	Repo   TrackRepository
}

func NewGetNowPlaying(client Navidrome, repo TrackRepository) *GetNowPlaying {
	return &GetNowPlaying{
		Client: client,
		Repo:   repo,
	}
}

func (i *GetNowPlaying) Execute(
	ctx context.Context,
) (*NowPlaying, error) {
	track, err := i.Client.NowPlaying(ctx)
	if err != nil {
		slog.Error("Cannot get now playing", "error", err)
		return nil, err
	}
	if track == nil {
		return nil, nil
	}

	nowPlaying := &NowPlaying{PlayingTrack: *track}

	file, err := i.Repo.FindTelegramFile(ctx, track.Metadata())
	switch {
	case err == nil:
		nowPlaying.TelegramFile = file
	case errors.Is(err, ErrTrackNotFound):
		slog.Info("now_playing_track_not_in_db", "title", track.Title, "artist", track.Artist)
	default:
		slog.Error("find_track", "error", err)
	}

	return nowPlaying, nil
}
