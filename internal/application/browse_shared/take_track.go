package browse_shared

import (
	"context"
	"errors"
	"time"

	"github.com/lubaskinc0de/beatstash/internal/application/common"
	"github.com/lubaskinc0de/beatstash/internal/application/common/libraries"
	"github.com/lubaskinc0de/beatstash/internal/application/common/quotas"
	"github.com/lubaskinc0de/beatstash/internal/application/common/repositories"
	"github.com/lubaskinc0de/beatstash/internal/domain/library"
)

var ErrAlreadyInLibrary = errors.New("the user has the track already")

type TakeTrack struct {
	IDs       common.IDProvider
	Tx        repositories.TxManager
	Lock      repositories.LibraryLock
	Tracks    repositories.Tracks
	Shared    repositories.SharedTracks
	Takes     repositories.Takes
	Libraries *libraries.Libraries
	Quotas    *quotas.Quotas
	Disk      common.Disk
	MusicDir  string
	Clock     func() time.Time
}

// Execute returns a *library.QuotaExceededError if the Track does not fit
// the taker's Personal Library.
func (i *TakeTrack) Execute(ctx context.Context, sharedTrackID uint) error {
	user, err := i.IDs.CurrentUser(ctx)
	if err != nil {
		return err
	}
	libs, err := i.Libraries.Of(ctx, user.ID)
	if err != nil {
		return err
	}
	return libraries.Within(ctx, i.Tx, i.Lock, i.Disk, libs.ManagedLibraries, func(ctx context.Context, changes *libraries.FileChanges) error {
		shared, err := i.Shared.Get(ctx, sharedTrackID)
		if err != nil {
			return err
		}
		track := shared.Track

		alreadyKept, err := i.Tracks.WithDuplicates(ctx, libs.KeptIDs(), []library.Track{*track})
		if err != nil {
			return err
		}
		if alreadyKept[track.ID] {
			return ErrAlreadyInLibrary
		}
		if err := i.Quotas.Admit(ctx, libs.Personal, track.Size); err != nil {
			return err
		}

		copied, _, err := libraries.CopyTrack(ctx, i.Tracks, i.Disk, i.MusicDir, track, libs.Shared, libs.Personal, changes)
		if err != nil {
			return err
		}
		return i.Takes.Save(ctx, shared.TakeBy(user, copied, i.Clock()))
	})
}
