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
	Disk      common.Disk
	MusicDir  string
}

func (i *UnshareAlbum) Execute(ctx context.Context, trackID uint) (*ShareState, error) {
	user, libs, err := libraries.Current(ctx, i.IDs, i.Libraries)
	if err != nil {
		return nil, err
	}
	var state *ShareState
	err = libraries.Within(ctx, i.Tx, i.Lock, i.Disk, libs, func(ctx context.Context, changes *libraries.FileChanges) error {
		op := &operation{
			tracks:   i.Tracks,
			shared:   i.Shared,
			disk:     i.Disk,
			musicDir: i.MusicDir,
			user:     user,
			libs:     libs,
			changes:  changes,
		}
		track, err := own(ctx, i.Tracks, libs.Personal, trackID)
		if err != nil {
			return err
		}
		tracks, err := album(ctx, i.Tracks, libs.Personal, track)
		if err != nil {
			return err
		}
		for j := range tracks {
			if err := op.unshare(ctx, &tracks[j]); err != nil {
				return err
			}
		}
		state, err = shareState(ctx, i.Tracks, i.Shared, libs.Personal, track)
		return err
	})
	return state, err
}
