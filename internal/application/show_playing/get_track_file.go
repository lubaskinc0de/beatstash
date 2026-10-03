package show_playing

import (
	"context"

	"github.com/lubaskinc0de/beatstash/internal/application/common"
	"github.com/lubaskinc0de/beatstash/internal/application/common/libraries"
	"github.com/lubaskinc0de/beatstash/internal/application/common/repositories"
	"github.com/lubaskinc0de/beatstash/internal/domain/library"
)

type GetTrackFile struct {
	IDs       common.IDProvider
	Tracks    repositories.Tracks
	Libraries *libraries.Libraries
}

func (i *GetTrackFile) Execute(ctx context.Context, trackID uint) (*library.Track, string, error) {
	user, err := i.IDs.CurrentUser(ctx)
	if err != nil {
		return nil, "", err
	}
	libs, err := i.Libraries.Of(ctx, user.ID)
	if err != nil {
		return nil, "", err
	}
	track, err := i.Tracks.Get(ctx, trackID)
	if err != nil {
		return nil, "", err
	}
	if err := track.AudibleBy(libs.Kept(), libs.Shared); err != nil {
		return nil, "", err
	}
	path, err := i.Libraries.FilePath(ctx, track)
	if err != nil {
		return nil, "", err
	}
	return track, path, nil
}
