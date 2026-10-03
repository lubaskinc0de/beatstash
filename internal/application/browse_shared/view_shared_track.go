package browse_shared

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

type ViewSharedTrack struct {
	IDs       common.IDProvider
	Shared    repositories.SharedTracks
	Tracks    repositories.Tracks
	Libraries *libraries.Libraries
}

// Execute returns sharing.ErrNotShared once the Track has left the Shared
// Library.
func (i *ViewSharedTrack) Execute(ctx context.Context, sharedTrackID uint) (*FeedEntry, error) {
	user, err := i.IDs.CurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	libs, err := i.Libraries.Of(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	shared, err := i.Shared.Get(ctx, sharedTrackID)
	if err != nil {
		return nil, err
	}
	alreadyKept, err := i.Tracks.WithDuplicates(ctx, libs.KeptIDs(), []library.Track{*shared.Track})
	if err != nil {
		return nil, err
	}
	return &FeedEntry{Track: *shared.Track, Author: shared.Author().User, InLibrary: alreadyKept[shared.Track.ID]}, nil
}
