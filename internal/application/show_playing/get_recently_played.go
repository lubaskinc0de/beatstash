package show_playing

import (
	"context"
	"log/slog"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/accounts"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/navidrome"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

type RecentTrack struct {
	navidrome.PlayedTrack
	// Track is nil if the user's libraries hold no such Track.
	Track *library.Track
}

type GetRecentlyPlayed struct {
	IDs       common.IDProvider
	Client    navidrome.Client
	Repo      repositories.Tracks
	Accounts  *accounts.Navidrome
	Libraries *libraries.Libraries
}

func (i *GetRecentlyPlayed) Execute(
	ctx context.Context,
	limit int,
) ([]RecentTrack, error) {
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

	played, err := i.Client.RecentlyPlayed(ctx, creds, limit)
	if err != nil {
		slog.Error("Cannot get recently played", "error", err)
		return nil, err
	}

	tracks := make([]RecentTrack, 0, len(played))
	for _, p := range played {
		track := RecentTrack{PlayedTrack: p}

		if track.Track, err = findPlayed(ctx, i.Repo, libs, p.Metadata()); err != nil {
			slog.Error("find_track", "error", err)
		}

		tracks = append(tracks, track)
	}

	return tracks, nil
}
