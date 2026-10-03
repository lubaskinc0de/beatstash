package browse_shared

import (
	"context"

	"github.com/lubaskinc0de/beatstash/internal/application/common"
	"github.com/lubaskinc0de/beatstash/internal/application/common/libraries"
	"github.com/lubaskinc0de/beatstash/internal/application/common/repositories"
	"github.com/lubaskinc0de/beatstash/internal/domain/library"
)

type GetTrackAudio struct {
	IDs       common.IDProvider
	Shared    repositories.SharedTracks
	Libraries *libraries.Libraries
}

func (i *GetTrackAudio) Execute(ctx context.Context, sharedTrackID uint) (*library.Track, string, error) {
	if _, err := i.IDs.CurrentUser(ctx); err != nil {
		return nil, "", err
	}
	shared, err := i.Shared.Get(ctx, sharedTrackID)
	if err != nil {
		return nil, "", err
	}
	path, err := i.Libraries.FilePath(ctx, shared.Track)
	if err != nil {
		return nil, "", err
	}
	return shared.Track, path, nil
}
