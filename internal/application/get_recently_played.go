package application

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

type RecentTrack struct {
	ID       string
	Artist   string
	Title    string
	Album    string
	Duration int
	PlayedAt time.Time

	TelegramFileID string
}

type GetRecentlyPlayed struct {
	Client *NavidromeClient
	Repo   TrackRepository
}

func NewGetRecentlyPlayed(client *NavidromeClient, repo TrackRepository) *GetRecentlyPlayed {
	return &GetRecentlyPlayed{
		Client: client,
		Repo:   repo,
	}
}

func (i *GetRecentlyPlayed) Execute(
	ctx context.Context,
	limit int,
) ([]RecentTrack, error) {
	songs, err := i.Client.GetRecentlyPlayed(ctx, limit)
	if err != nil {
		slog.Error("Cannot get recently played", "error", err)
		return nil, err
	}

	tracks := make([]RecentTrack, 0, len(songs))
	for _, song := range songs {
		track := RecentTrack{
			ID:       song.ID,
			Artist:   song.Artist,
			Title:    song.Title,
			Album:    song.Album,
			Duration: int(song.Duration),
			PlayedAt: *song.PlayDate,
		}

		saved, err := i.Repo.FindByTitleAndPerformer(ctx, song.Title, song.Artist)
		switch {
		case err == nil:
			track.TelegramFileID = saved.TelegramFileID
		case errors.Is(err, ErrTrackNotFound):
		default:
			slog.Error("find_track", "error", err)
		}

		tracks = append(tracks, track)
	}

	return tracks, nil
}
