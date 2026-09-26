package browse_shared

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

type FeedEntry struct {
	Track  library.Track
	Author access.User
}

type ViewFeed struct {
	IDs    common.IDProvider
	Shares repositories.Shares
}

func (i *ViewFeed) Execute(ctx context.Context, limit int) ([]FeedEntry, error) {
	if _, err := i.IDs.CurrentUser(ctx); err != nil {
		return nil, err
	}
	shares, err := i.Shares.Feed(ctx, limit)
	if err != nil {
		return nil, err
	}

	entries := make([]FeedEntry, 0, len(shares))
	for _, share := range shares {
		entries = append(entries, FeedEntry{Track: share.Track, Author: share.User})
	}
	return entries, nil
}
