package share_tracks

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
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

func (i *UnshareTrack) Execute(ctx context.Context, trackID uint) (*ShareState, error) {
	_, libs, err := libraries.CurrentManaged(ctx, i.IDs, i.Libraries)
	if err != nil {
		return nil, err
	}
	var state *ShareState
	err = libraries.Within(ctx, i.Tx, i.Lock, i.Disk, libs, func(ctx context.Context, changes *libraries.FileChanges) error {
		sharedDir := libraries.Dir(i.MusicDir, libs.Shared)
		personal := []*library.Library{libs.Personal}
		track, err := keptTrack(ctx, i.Tracks, personal, trackID)
		if err != nil {
			return err
		}
		if err := unshare(ctx, i.Shared, sharedDir, changes, track); err != nil {
			return err
		}
		albumTracks, err := album(ctx, i.Tracks, personal, track)
		if err != nil {
			return err
		}
		state, err = shareState(ctx, i.Shared, track, albumTracks)
		return err
	})
	return state, err
}
