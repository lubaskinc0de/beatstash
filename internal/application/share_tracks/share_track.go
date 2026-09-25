package share_tracks

import (
	"context"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
)

type ShareTrack struct {
	IDs       common.IDProvider
	Tx        repositories.TxManager
	Lock      repositories.LibraryLock
	Tracks    repositories.Tracks
	Shares    repositories.Shares
	Libraries *libraries.Libraries
	Disk      common.Disk
	MusicDir  string
	Clock     func() time.Time
}

func (i *ShareTrack) Execute(ctx context.Context, trackID uint) (*ShareResult, error) {
	user, libs, err := libraries.Current(ctx, i.IDs, i.Libraries)
	if err != nil {
		return nil, err
	}
	result := &ShareResult{}
	err = libraries.Within(ctx, i.Tx, i.Lock, libs, func(ctx context.Context, changes *libraries.FileChanges) error {
		op := &operation{
			tracks:   i.Tracks,
			shares:   i.Shares,
			disk:     i.Disk,
			musicDir: i.MusicDir,
			user:     user,
			libs:     libs,
			changes:  changes,
			now:      i.Clock(),
		}
		track, err := own(ctx, i.Tracks, libs.Personal, trackID)
		if err != nil {
			return err
		}
		if err := op.share(ctx, track, result); err != nil {
			return err
		}
		result.State, err = shareState(ctx, i.Tracks, i.Shares, libs.Personal, track)
		return err
	})
	return result, err
}
