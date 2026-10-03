package share_tracks

import (
	"context"

	"github.com/lubaskinc0de/beatstash/internal/application/common"
	"github.com/lubaskinc0de/beatstash/internal/application/common/libraries"
	"github.com/lubaskinc0de/beatstash/internal/application/common/repositories"
)

type UnshareTrack struct {
	IDs       common.IDProvider
	Tx        repositories.TxManager
	Lock      repositories.LibraryLock
	Tracks    repositories.Tracks
	Shared    repositories.SharedTracks
	Libraries *libraries.Libraries
	Disk      common.Disk
	MusicDir  string
}

func (i *UnshareTrack) Execute(ctx context.Context, trackID uint) error {
	user, err := i.IDs.CurrentUser(ctx)
	if err != nil {
		return err
	}
	libs, err := i.Libraries.Of(ctx, user.ID)
	if err != nil {
		return err
	}
	return libraries.Within(ctx, i.Tx, i.Lock, i.Disk, libs.ManagedLibraries, func(ctx context.Context, changes *libraries.FileChanges) error {
		track, err := keptTrack(ctx, i.Tracks, libs.Kept(), trackID)
		if err != nil {
			return err
		}
		return unshare(ctx, i.Shared, libraries.Dir(i.MusicDir, libs.Shared), changes, track)
	})
}
