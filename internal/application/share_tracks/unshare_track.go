package share_tracks

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
)

type UnshareTrack struct {
	IDs       common.IDProvider
	Tx        repositories.TxManager
	Lock      repositories.LibraryLock
	Tracks    repositories.Tracks
	Shared    repositories.SharedTracks
	Libraries *libraries.Libraries
	Attached  *libraries.Attached
	Disk      common.Disk
	MusicDir  string
}

func (i *UnshareTrack) Execute(ctx context.Context, trackID uint) error {
	_, libs, kept, err := libraries.CurrentKept(ctx, i.IDs, i.Libraries, i.Attached)
	if err != nil {
		return err
	}
	return libraries.Within(ctx, i.Tx, i.Lock, i.Disk, libs, func(ctx context.Context, changes *libraries.FileChanges) error {
		track, err := keptTrack(ctx, i.Tracks, kept, trackID)
		if err != nil {
			return err
		}
		return unshare(ctx, i.Shared, libraries.Dir(i.MusicDir, libs.Shared), changes, track)
	})
}
