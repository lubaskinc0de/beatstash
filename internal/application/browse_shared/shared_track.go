package browse_shared

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/sharing"
)

func sharedTrack(ctx context.Context, tracks repositories.Tracks, shared *library.Library, trackID uint) (*library.Track, error) {
	track, err := tracks.Get(ctx, trackID)
	if errors.Is(err, repositories.ErrTrackNotFound) {
		return nil, sharing.ErrNotShared
	}
	if err != nil {
		return nil, err
	}
	if !track.In(shared) {
		return nil, sharing.ErrNotShared
	}
	return track, nil
}
