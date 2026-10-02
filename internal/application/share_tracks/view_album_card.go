package share_tracks

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/listening"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

type AlbumCard struct {
	Album   library.AlbumKey
	Library *library.Library
	// Tracks are only those the user can share.
	Tracks   []library.Track
	Shared   map[uint]bool
	Linkable bool
}

func (c *AlbumCard) SharedCount() int {
	n := 0
	for _, t := range c.Tracks {
		if c.Shared[t.ID] {
			n++
		}
	}
	return n
}

func (c *AlbumCard) AllShared() bool {
	return c.SharedCount() == len(c.Tracks)
}

type ViewAlbumCard struct {
	IDs         common.IDProvider
	Tracks      repositories.Tracks
	Shared      repositories.SharedTracks
	Libraries   *libraries.Libraries
	Attached    *libraries.Attached
	ListenLinks *listening.ListenLinks
}

func (i *ViewAlbumCard) Execute(ctx context.Context, trackID uint) (*AlbumCard, error) {
	user, _, kept, err := libraries.CurrentKept(ctx, i.IDs, i.Libraries, i.Attached)
	if err != nil {
		return nil, err
	}
	track, err := keptTrack(ctx, i.Tracks, kept, trackID)
	if err != nil {
		return nil, err
	}
	card := &AlbumCard{Album: track.AlbumKey(), Library: libraryOf(track, kept)}
	card.Tracks, err = album(ctx, i.Tracks, kept, track)
	if err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(card.Tracks))
	for _, t := range card.Tracks {
		ids = append(ids, t.ID)
	}
	card.Shared, err = i.Shared.SharedSources(ctx, ids)
	if err != nil {
		return nil, err
	}
	card.Linkable, err = i.ListenLinks.Available(ctx, user.ID)
	return card, err
}
