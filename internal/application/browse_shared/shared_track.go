package browse_shared

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

func sharedTrack(ctx context.Context, tracks repositories.Tracks, shared *domain.Library, trackID uint) (*domain.Track, error) {
	track, err := tracks.Get(ctx, trackID)
	if errors.Is(err, repositories.ErrTrackNotFound) {
		return nil, domain.ErrNotShared
	}
	if err != nil {
		return nil, err
	}
	if !track.In(shared) {
		return nil, domain.ErrNotShared
	}
	return track, nil
}
