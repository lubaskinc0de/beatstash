package browse_shared

import (
	"context"

	"github.com/lubaskinc0de/beatstash/internal/application/common"
	"github.com/lubaskinc0de/beatstash/internal/application/common/libraries"
	"github.com/lubaskinc0de/beatstash/internal/application/common/repositories"
	"github.com/lubaskinc0de/beatstash/internal/domain/access"
	"github.com/lubaskinc0de/beatstash/internal/domain/library"
)

// TrackAudio is the audio of a Shared Library Track with who shared it.
type TrackAudio struct {
	Track  *library.Track
	Author access.User
	Path   string
}

type GetTrackAudio struct {
	IDs       common.IDProvider
	Shared    repositories.SharedTracks
	Libraries *libraries.Libraries
}

// Execute returns sharing.ErrNotShared once the Track has left the Shared
// Library.
func (i *GetTrackAudio) Execute(ctx context.Context, sharedTrackID uint) (*TrackAudio, error) {
	if _, err := i.IDs.CurrentUser(ctx); err != nil {
		return nil, err
	}
	shared, err := i.Shared.Get(ctx, sharedTrackID)
	if err != nil {
		return nil, err
	}
	path, err := i.Libraries.FilePath(ctx, shared.Track)
	if err != nil {
		return nil, err
	}
	return &TrackAudio{Track: shared.Track, Author: shared.Author().User, Path: path}, nil
}
