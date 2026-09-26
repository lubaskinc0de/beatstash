package share_tracks

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
)

type ShowShareOptions struct {
	IDs       common.IDProvider
	Tracks    repositories.Tracks
	Shared    repositories.SharedTracks
	Libraries *libraries.Libraries
}

func (i *ShowShareOptions) Execute(ctx context.Context, ref provider.TrackRef) (*ShareState, error) {
	_, libs, err := libraries.Current(ctx, i.IDs, i.Libraries)
	if err != nil {
		return nil, err
	}
	source, err := i.Tracks.FindSource(ctx, libs.Personal.ID, ref.Provider, ref.ID)
	if err != nil {
		return nil, err
	}
	track, err := own(ctx, i.Tracks, libs.Personal, source.TrackID)
	if err != nil {
		return nil, err
	}
	if err := track.ShareableBy(libs.Personal); err != nil {
		return nil, err
	}
	return shareState(ctx, i.Tracks, i.Shared, libs.Personal, track)
}
