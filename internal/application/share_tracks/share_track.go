package share_tracks

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

type ShareTrack struct {
	ShareDeps
}

func (i *ShareTrack) Execute(ctx context.Context, trackID uint) (*ShareResult, error) {
	result := &ShareResult{}
	err := i.within(ctx, func(ctx context.Context, s *sharer) error {
		track, err := keptTrack(ctx, i.Tracks, s.kept, trackID)
		if err != nil {
			return err
		}
		locked, err := s.lockAttached(ctx, []library.Track{*track})
		if err != nil {
			return err
		}
		for j := range locked {
			if err := s.share(ctx, &locked[j], result); err != nil {
				return err
			}
		}
		albumTracks, err := album(ctx, i.Tracks, s.kept, track)
		if err != nil {
			return err
		}
		result.State, err = shareState(ctx, i.Shared, track, albumTracks)
		return err
	})
	return result, err
}
