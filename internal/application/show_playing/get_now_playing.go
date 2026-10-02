package show_playing

import (
	"context"
	"log/slog"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/accounts"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/listening"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/navidrome"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

type NowPlaying struct {
	navidrome.PlayingTrack
	// Track is nil if the user's libraries hold no such Track.
	Track *library.Track
	// Linkable: the user can send the Track's Listen Link; they have a
	// Navidrome Account, or nothing would be playing.
	Linkable bool
}

type GetNowPlaying struct {
	IDs         common.IDProvider
	Client      navidrome.Client
	Tracks      repositories.Tracks
	Accounts    *accounts.Navidrome
	Libraries   *libraries.Libraries
	Attached    *libraries.Attached
	ListenLinks *listening.ListenLinks
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

	found, err := findTracks(ctx, i.Tracks, libs.IDs(), attached, []navidrome.Track{track.Track})
	if err != nil {
		slog.Error("find_track", "error", err)
		return nowPlaying, nil
	}
	nowPlaying.Track = found[0]
	nowPlaying.Linkable = i.ListenLinks.On
	return nowPlaying, nil
}
