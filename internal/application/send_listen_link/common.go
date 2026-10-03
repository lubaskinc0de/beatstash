package send_listen_link

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/navidrome"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

var ErrNotIndexed = errors.New("not indexed by navidrome yet")

func audible(
	ctx context.Context,
	ids common.IDProvider,
	libraries *libraries.Libraries,
	tracks repositories.Tracks,
	trackID uint,
) (*access.User, *library.Track, error) {
	user, err := ids.CurrentUser(ctx)
	if err != nil {
		return nil, nil, err
	}
	libs, err := libraries.Of(ctx, user.ID)
	if err != nil {
		return nil, nil, err
	}
	track, err := tracks.Get(ctx, trackID)
	if errors.Is(err, repositories.ErrTrackNotFound) {
		return nil, nil, library.ErrNotKeptTrack
	}
	if err != nil {
		return nil, nil, err
	}
	return user, track, track.AudibleBy(libs.Kept(), libs.Shared)
}

// songOf asks Navidrome, as admin, for a Track whose song is not known yet.
func songOf(
	ctx context.Context,
	tracks repositories.Tracks,
	libs repositories.Libraries,
	client navidrome.Client,
	admin navidrome.Credentials,
	track *library.Track,
) (string, error) {
	if track.Indexed() {
		return track.SongID, nil
	}
	lib, err := libs.Get(ctx, track.LibraryID)
	if err != nil {
		return "", err
	}
	if lib.NavidromeID == 0 {
		return "", ErrNotIndexed
	}
	songID, err := client.SongAt(ctx, admin, lib.NavidromeID, track.Path)
	if err != nil {
		return "", err
	}
	if songID == "" {
		return "", ErrNotIndexed
	}
	track.IndexedAs(songID)
	return songID, tracks.SetSongs(ctx, []library.Track{*track})
}

func description(artist, title string) string {
	return artist + " — " + title
}
