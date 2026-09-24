package application

import (
	"context"
	"errors"
	"log/slog"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type RecentTrack struct {
	PlayedTrack
	TelegramFile *domain.TelegramFile
}

type GetRecentlyPlayed struct {
	Client    Navidrome
	Repo      TrackRepository
	Accounts  *NavidromeAccounts
	Libraries *Libraries
}

func NewGetRecentlyPlayed(client Navidrome, repo TrackRepository, accounts *NavidromeAccounts, libraries *Libraries) *GetRecentlyPlayed {
	return &GetRecentlyPlayed{
		Client:    client,
		Repo:      repo,
		Accounts:  accounts,
		Libraries: libraries,
	}
}

func (i *GetRecentlyPlayed) Execute(
	ctx context.Context,
	limit int,
) ([]RecentTrack, error) {
	creds, err := i.Accounts.Credentials(ctx)
	if err != nil {
		return nil, err
	}
	user, _ := UserFromContext(ctx)
	libs, err := i.Libraries.Of(ctx, user)
	if err != nil {
		return nil, err
	}

	played, err := i.Client.RecentlyPlayed(ctx, creds, limit)
	if err != nil {
		slog.Error("Cannot get recently played", "error", err)
		return nil, err
	}

	tracks := make([]RecentTrack, 0, len(played))
	for _, p := range played {
		track := RecentTrack{PlayedTrack: p}

		file, err := i.Repo.FindTelegramFile(ctx, libs.IDs(), p.Metadata())
		switch {
		case err == nil:
			track.TelegramFile = file
		case errors.Is(err, ErrTrackNotFound):
		default:
			slog.Error("find_track", "error", err)
		}

		tracks = append(tracks, track)
	}

	return tracks, nil
}
