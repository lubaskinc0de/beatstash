package browse_shared

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

type GetTrackAudio struct {
	IDs       common.IDProvider
	Tracks    repositories.Tracks
	Libraries *libraries.Libraries
}

func (i *GetTrackAudio) Execute(ctx context.Context, sharedTrackID uint) (*library.Track, string, error) {
	if _, err := i.IDs.CurrentUser(ctx); err != nil {
		return nil, "", err
	}
	shared, err := i.Libraries.Shared(ctx)
	if err != nil {
		return nil, "", err
	}
	track, err := sharedTrack(ctx, i.Tracks, shared, sharedTrackID)
	if err != nil {
		return nil, "", err
	}
	path, err := i.Libraries.FilePath(ctx, track)
	if err != nil {
		return nil, "", err
	}
	return track, path, nil
}
