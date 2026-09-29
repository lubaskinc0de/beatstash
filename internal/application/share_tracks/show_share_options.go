package share_tracks

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
)

type ShowShareOptions struct {
	IDs       common.IDProvider
	Tracks    repositories.Tracks
	Shared    repositories.SharedTracks
	Libraries *libraries.Libraries
}

func (i *ShowShareOptions) Execute(ctx context.Context, ref provider.TrackRef) (*ShareState, error) {
	_, libs, err := libraries.CurrentManaged(ctx, i.IDs, i.Libraries)
	if err != nil {
		return nil, err
	}
	source, err := i.Tracks.FindSource(ctx, []uint{libs.Personal.ID}, ref.Provider, ref.ID)
	if err != nil {
		return nil, err
	}
	personal := []*library.Library{libs.Personal}
	track, err := keptTrack(ctx, i.Tracks, personal, source.TrackID)
	if err != nil {
		return nil, err
	}
	if err := track.ShareableBy(personal); err != nil {
		return nil, err
	}
	albumTracks, err := album(ctx, i.Tracks, personal, track)
	if err != nil {
		return nil, err
	}
	return shareState(ctx, i.Shared, track, albumTracks)
}
