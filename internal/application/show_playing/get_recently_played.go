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

type RecentTrack struct {
	navidrome.PlayedTrack
	// Track is nil if the user's libraries hold no such Track.
	Track *library.Track
}

type RecentlyPlayed struct {
	Tracks []RecentTrack
	// Linkable: the user can send the Tracks' Listen Links; they have a
	// Navidrome Account, or nothing would be played.
	Linkable bool
}

type GetRecentlyPlayed struct {
	IDs         common.IDProvider
	Client      navidrome.Client
	Tracks      repositories.Tracks
	Accounts    *accounts.Navidrome
	Libraries   *libraries.Libraries
	ListenLinks *listening.ListenLinks
}

func (i *GetRecentlyPlayed) Execute(
	ctx context.Context,
	limit int,
) (*RecentlyPlayed, error) {
	user, err := i.IDs.CurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	creds, err := i.Accounts.Credentials(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	played, err := i.Client.RecentlyPlayed(ctx, creds, limit)
	if err != nil {
		return nil, err
	}

	libs, err := i.Libraries.Of(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	songs := make([]navidrome.Track, 0, len(played))
	for _, p := range played {
		songs = append(songs, p.Track)
	}
	found, err := findTracks(ctx, i.Tracks, libs.IDs(), libraries.IDs(libs.Attached), songs)
	if err != nil {
		slog.Error("find_track", "error", err)
		found = make([]*library.Track, len(played))
	}
	recent := &RecentlyPlayed{Tracks: make([]RecentTrack, 0, len(played))}
	for n, p := range played {
		recent.Tracks = append(recent.Tracks, RecentTrack{PlayedTrack: p, Track: found[n]})
	}
	recent.Linkable = i.ListenLinks.On
	return recent, nil
}
