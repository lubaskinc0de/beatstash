package show_playing

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

type GetTrackFile struct {
	IDs       common.IDProvider
	Repo      repositories.Tracks
	Libraries *libraries.Libraries
	Attached  *libraries.Attached
}

func (i *GetTrackFile) Execute(ctx context.Context, trackID uint) (*library.Track, string, error) {
	_, libs, kept, err := libraries.CurrentKept(ctx, i.IDs, i.Libraries, i.Attached)
	if err != nil {
		return nil, "", err
	}
	track, err := i.Repo.Get(ctx, trackID)
	if err != nil {
		return nil, "", err
	}
	if err := track.AudibleBy(kept, libs.Shared); err != nil {
		return nil, "", err
	}
	path, err := i.Libraries.FilePath(ctx, track)
	if err != nil {
		return nil, "", err
	}
	return track, path, nil
}
