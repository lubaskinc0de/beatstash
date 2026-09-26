package browse_shared

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

type FeedEntry struct {
	Track  library.Track
	Author access.User
	// InLibrary: the viewer's Personal Library has this Track, so Take
	// would refuse it.
	InLibrary bool
}

type ViewFeed struct {
	IDs       common.IDProvider
	Shared    repositories.SharedTracks
	Tracks    repositories.Tracks
	Libraries *libraries.Libraries
}

func (i *ViewFeed) Execute(ctx context.Context, limit int) ([]FeedEntry, error) {
	_, libs, err := libraries.Current(ctx, i.IDs, i.Libraries)
	if err != nil {
		return nil, err
	}
	shares, err := i.Shared.Feed(ctx, limit)
	if err != nil {
		return nil, err
	}

	entries := make([]FeedEntry, 0, len(shares))
	for _, share := range shares {
		_, err := i.Tracks.FindDuplicate(ctx, libs.Personal.ID, share.Track.Metadata, share.Track.DurationMs)
		if err != nil && !errors.Is(err, repositories.ErrTrackNotFound) {
			return nil, err
		}
		entries = append(entries, FeedEntry{Track: share.Track, Author: share.User, InLibrary: err == nil})
	}
	return entries, nil
}
