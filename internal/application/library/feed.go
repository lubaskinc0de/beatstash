package library

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/application"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type FeedEntry struct {
	Track        domain.Track
	Author       domain.User
	TelegramFile *domain.TelegramFile
}

func (s *Sharing) Feed(ctx context.Context, limit int) ([]FeedEntry, error) {
	shares, err := s.Shares.Feed(ctx, limit)
	if err != nil {
		return nil, err
	}

	entries := make([]FeedEntry, 0, len(shares))
	for _, share := range shares {
		entry := FeedEntry{Track: share.Track, Author: share.User}
		file, err := s.Tracks.TelegramFile(ctx, share.TrackID)
		switch {
		case err == nil:
			entry.TelegramFile = file
		case !errors.Is(err, application.ErrTrackNotFound):
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// SharedFile returns ErrTrackNotFound when the Track has no Telegram file.
func (s *Sharing) SharedFile(ctx context.Context, sharedTrackID uint) (*domain.TelegramFile, error) {
	shared, err := s.Libraries.Shared(ctx)
	if err != nil {
		return nil, err
	}
	track, err := s.sharedTrack(ctx, shared, sharedTrackID)
	if err != nil {
		return nil, err
	}
	return s.Tracks.TelegramFile(ctx, track.ID)
}
