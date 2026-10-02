package share_tracks

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
)

type UnshareAlbum struct {
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

func (i *UnshareAlbum) Execute(ctx context.Context, trackID uint) (*ShareState, error) {
	_, libs, kept, err := libraries.CurrentKept(ctx, i.IDs, i.Libraries, i.Attached)
	if err != nil {
		return nil, err
	}
	var state *ShareState
	err = libraries.Within(ctx, i.Tx, i.Lock, i.Disk, libs, func(ctx context.Context, changes *libraries.FileChanges) error {
		sharedDir := libraries.Dir(i.MusicDir, libs.Shared)
		track, err := keptTrack(ctx, i.Tracks, kept, trackID)
		if err != nil {
			return err
		}
		tracks, err := album(ctx, i.Tracks, kept, track)
		if err != nil {
			return err
		}
		for j := range tracks {
			if err := unshare(ctx, i.Shared, sharedDir, changes, &tracks[j]); err != nil {
				return err
			}
		}
		state, err = shareState(ctx, i.Shared, track, tracks)
		return err
	})
	return state, err
}
