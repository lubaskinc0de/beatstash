package browse_shared

import (
	"context"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/quotas"
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
	Attached  *libraries.Attached
	Quotas    *quotas.Quotas
	Disk      common.Disk
	MusicDir  string
	Clock     func() time.Time
}

// Execute returns a *library.QuotaExceededError if the Track does not fit
// the taker's Personal Library.
func (i *TakeTrack) Execute(ctx context.Context, sharedTrackID uint) error {
	user, libs, kept, err := libraries.CurrentKept(ctx, i.IDs, i.Libraries, i.Attached)
	if err != nil {
		return err
	}
	return libraries.Within(ctx, i.Tx, i.Lock, i.Disk, libs, func(ctx context.Context, changes *libraries.FileChanges) error {
		shared, err := i.Shared.Get(ctx, sharedTrackID)
		if err != nil {
			return err
		}
		track := shared.Track

		alreadyKept, err := i.Tracks.WithDuplicates(ctx, libraries.IDs(kept), []library.Track{*track})
		if err != nil {
			return err
		}
		if alreadyKept[track.ID] {
			return library.ErrAlreadyInLibrary
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
