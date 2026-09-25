package browse_shared

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type FeedEntry struct {
	Track        domain.Track
	Author       domain.User
	TelegramFile *domain.TelegramFile
}

type ViewFeed struct {
	IDs    common.IDProvider
	Tracks repositories.Tracks
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
		entry := FeedEntry{Track: share.Track, Author: share.User}
		file, err := i.Tracks.TelegramFile(ctx, share.TrackID)
		switch {
		case err == nil:
			entry.TelegramFile = file
		case !errors.Is(err, repositories.ErrNoTelegramFile):
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}
