package browse_shared

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

type FeedEntry struct {
	Track  library.Track
	Author access.User
	// InLibrary: the viewer's Personal Library or an Attached Library they
	// see has this Track, so Take would refuse it.
	InLibrary bool
}

type ViewFeed struct {
	IDs       common.IDProvider
	Shared    repositories.SharedTracks
	Tracks    repositories.Tracks
	Libraries *libraries.Libraries
	Attached  *libraries.Attached
}

func (i *ViewFeed) Execute(ctx context.Context, limit int) ([]FeedEntry, error) {
	_, _, kept, err := libraries.CurrentKept(ctx, i.IDs, i.Libraries, i.Attached)
	if err != nil {
		return nil, err
	}
	shares, err := i.Shared.Feed(ctx, limit)
	if err != nil {
		return nil, err
	}

	sharedTracks := make([]library.Track, 0, len(shares))
	for _, share := range shares {
		sharedTracks = append(sharedTracks, share.Track)
	}
	alreadyKept, err := i.Tracks.WithDuplicates(ctx, libraries.IDs(kept), sharedTracks)
	if err != nil {
		return nil, err
	}
	entries := make([]FeedEntry, 0, len(shares))
	for _, share := range shares {
		entries = append(entries, FeedEntry{Track: share.Track, Author: share.User, InLibrary: alreadyKept[share.Track.ID]})
	}
	return entries, nil
}
