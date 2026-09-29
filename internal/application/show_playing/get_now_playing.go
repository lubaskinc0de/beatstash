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
	// ShareableTrackID is the playing Track of the user's Personal Library,
	// or of an Attached Library they see, that is not in the Shared Library
	// yet; zero means np offers no Share.
	ShareableTrackID uint
}

type GetNowPlaying struct {
	IDs       common.IDProvider
	Client    navidrome.Client
	Repo      repositories.Tracks
	Accounts  *accounts.Navidrome
	Libraries *libraries.Libraries
	Attached  *libraries.Attached
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

	visible, err := i.Attached.VisibleTo(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	attached := libraries.IDs(visible)
	nowPlaying := &NowPlaying{PlayingTrack: *track}

	found, err := findTracks(ctx, i.Repo, libs.IDs(), attached, []navidrome.Track{track.Track})
	if err != nil {
		slog.Error("find_track", "error", err)
		return nowPlaying, nil
	}
	nowPlaying.Track = found[0]

	nowPlaying.ShareableTrackID, err = i.shareable(ctx, libs.Shared, nowPlaying.Track)
	if err != nil {
		slog.Error("find_shareable_track", "error", err)
	}
	return nowPlaying, nil
}

// shareable is the playing Track unless it is in the Shared Library or has
// a Duplicate there.
func (i *GetNowPlaying) shareable(ctx context.Context, shared *library.Library, track *library.Track) (uint, error) {
	if track == nil || track.In(shared) {
		return 0, nil
	}
	_, err := i.Repo.FindDuplicate(ctx, []uint{shared.ID}, track.Metadata, track.DurationMs)
	switch {
	case err == nil:
		return 0, nil
	case errors.Is(err, repositories.ErrTrackNotFound):
		return track.ID, nil
	default:
		return 0, err
	}
}
