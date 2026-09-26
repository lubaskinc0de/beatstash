package show_playing

import (
	"context"
	"errors"
	"log/slog"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/accounts"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/navidrome"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

type NowPlaying struct {
	navidrome.PlayingTrack
	// Track is nil if the user's libraries hold no such Track.
	Track *library.Track
	// ShareableTrackID is the playing Track of the user's Personal Library
	// that is not in the Shared Library yet; zero means np offers no Share.
	ShareableTrackID uint
}

type GetNowPlaying struct {
	IDs       common.IDProvider
	Client    navidrome.Client
	Repo      repositories.Tracks
	Accounts  *accounts.Navidrome
	Libraries *libraries.Libraries
}

func (i *GetNowPlaying) Execute(
	ctx context.Context,
) (*NowPlaying, error) {
	user, err := i.IDs.CurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	creds, err := i.Accounts.Credentials(ctx, user.ID)
	if err != nil {
		return nil, err
	}
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

	nowPlaying.Track, err = findPlayed(ctx, i.Repo, libs, track.Metadata())
	if err != nil {
		slog.Error("find_track", "error", err)
	}

	nowPlaying.ShareableTrackID, err = i.shareable(ctx, libs, track.Metadata())
	if err != nil {
		slog.Error("find_shareable_track", "error", err)
	}
	return nowPlaying, nil
}

func (i *GetNowPlaying) shareable(ctx context.Context, libs libraries.UserLibraries, m library.Metadata) (uint, error) {
	own, err := i.Repo.FindByMetadata(ctx, []uint{libs.Personal.ID}, m)
	if errors.Is(err, repositories.ErrTrackNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	_, err = i.Repo.FindDuplicate(ctx, libs.Shared.ID, own.Metadata, own.DurationMs)
	switch {
	case err == nil:
		return 0, nil
	case errors.Is(err, repositories.ErrTrackNotFound):
		return own.ID, nil
	default:
		return 0, err
	}
}

func findPlayed(ctx context.Context, tracks repositories.Tracks, libs libraries.UserLibraries, m library.Metadata) (*library.Track, error) {
	track, err := tracks.FindByMetadata(ctx, libs.IDs(), m)
	if errors.Is(err, repositories.ErrTrackNotFound) {
		return nil, nil
	}
	return track, err
}
