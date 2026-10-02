package resolve_songs

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/navidrome"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

// ResolveSongs: search shows only an Album Navidrome has indexed, and a
// Listen Link leads to a song.
type ResolveSongs struct {
	Libraries repositories.Libraries
	Tracks    repositories.Tracks
	Navidrome navidrome.Client
	Admin     navidrome.Credentials
}

func (i *ResolveSongs) Execute(ctx context.Context) error {
	all, err := i.Libraries.All(ctx)
	if err != nil {
		return err
	}
	managed := map[uint]int{}
	ids := make([]uint, 0, len(all))
	for _, lib := range all {
		if !lib.Attached() && lib.NavidromeID != 0 {
			managed[lib.ID] = lib.NavidromeID
			ids = append(ids, lib.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	unindexed, err := i.Tracks.Unindexed(ctx, ids)
	if err != nil {
		return err
	}
	var found []library.Track
	for libraryID, tracks := range unindexed {
		songs, err := i.Navidrome.Songs(ctx, i.Admin, managed[libraryID])
		if err != nil {
			return err
		}
		for _, track := range tracks {
			if song := songs[track.Path]; song != "" {
				track.IndexedAs(song)
				found = append(found, track)
			}
		}
	}
	return i.Tracks.SetSongs(ctx, found)
}
