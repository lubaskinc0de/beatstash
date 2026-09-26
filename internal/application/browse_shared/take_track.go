package browse_shared

import (
	"context"
	"errors"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

type TakeTrack struct {
	IDs       common.IDProvider
	Tx        repositories.TxManager
	Lock      repositories.LibraryLock
	Tracks    repositories.Tracks
	Shared    repositories.SharedTracks
	Takes     repositories.Takes
	Libraries *libraries.Libraries
	Disk      common.Disk
	MusicDir  string
	Clock     func() time.Time
}

func (i *TakeTrack) Execute(ctx context.Context, sharedTrackID uint) error {
	user, libs, err := libraries.Current(ctx, i.IDs, i.Libraries)
	if err != nil {
		return err
	}
	return libraries.Within(ctx, i.Tx, i.Lock, i.Disk, libs, func(ctx context.Context, changes *libraries.FileChanges) error {
		shared, err := i.Shared.Get(ctx, sharedTrackID)
		if err != nil {
			return err
		}
		track := shared.Track

		_, err = i.Tracks.FindDuplicate(ctx, libs.Personal.ID, track.Metadata, track.DurationMs)
		if err == nil {
			return library.ErrAlreadyInLibrary
		}
		if !errors.Is(err, repositories.ErrTrackNotFound) {
			return err
		}

		copied, _, err := libraries.CopyTrack(ctx, i.Tracks, i.Disk, i.MusicDir, track, libs.Shared, libs.Personal, changes)
		if err != nil {
			return err
		}
		return i.Takes.Save(ctx, shared.TakeBy(user, copied, i.Clock()))
	})
}
