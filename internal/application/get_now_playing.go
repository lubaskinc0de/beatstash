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
	// ShareableTrackID is the playing Track of the user's Personal Library
	// that is not in the Shared Library yet; zero means np offers no Share.
	ShareableTrackID uint
}

type GetNowPlaying struct {
	Client    Navidrome
	Repo      TrackRepository
	Accounts  *NavidromeAccounts
	Libraries *Libraries
}

func NewGetNowPlaying(client Navidrome, repo TrackRepository, accounts *NavidromeAccounts, libraries *Libraries) *GetNowPlaying {
	return &GetNowPlaying{
		Client:    client,
		Repo:      repo,
		Accounts:  accounts,
		Libraries: libraries,
	}
}

func (i *GetNowPlaying) Execute(
	ctx context.Context,
) (*NowPlaying, error) {
	creds, err := i.Accounts.Credentials(ctx)
	if err != nil {
		return nil, err
	}
	user, _ := UserFromContext(ctx)
	libs, err := i.Libraries.Of(ctx, user)
	if err != nil {
		return nil, err
	}

	track, err := i.Client.NowPlaying(ctx, creds)
	if err != nil {
		slog.Error("Cannot get now playing", "error", err)
		return nil, err
	}
	if track == nil {
		return nil, nil
	}

	nowPlaying := &NowPlaying{PlayingTrack: *track}

	file, err := i.Repo.FindTelegramFile(ctx, libs.IDs(), track.Metadata())
	switch {
	case err == nil:
		nowPlaying.TelegramFile = file
	case errors.Is(err, ErrTrackNotFound):
		slog.Info("now_playing_track_not_in_db", "title", track.Title, "artist", track.Artist)
	default:
		slog.Error("find_track", "error", err)
	}

	nowPlaying.ShareableTrackID, err = i.shareable(ctx, libs, track.Metadata())
	if err != nil {
		slog.Error("find_shareable_track", "error", err)
	}
	return nowPlaying, nil
}

func (i *GetNowPlaying) shareable(ctx context.Context, libs UserLibraries, m domain.Metadata) (uint, error) {
	own, err := i.Repo.FindByMetadata(ctx, libs.Personal.ID, m)
	if errors.Is(err, ErrTrackNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	_, err = i.Repo.FindDuplicate(ctx, libs.Shared.ID, own.Metadata, own.DurationMs)
	switch {
	case err == nil:
		return 0, nil
	case errors.Is(err, ErrTrackNotFound):
		return own.ID, nil
	default:
		return 0, err
	}
}
