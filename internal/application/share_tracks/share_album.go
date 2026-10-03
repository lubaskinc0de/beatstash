package share_tracks

import (
	"context"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/navidrome"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/quotas"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
)

type ShareAlbum struct {
	IDs       common.IDProvider
	Tx        repositories.TxManager
	Lock      repositories.LibraryLock
	Tracks    repositories.Tracks
	Shared    repositories.SharedTracks
	Libraries *libraries.Libraries
	Quotas    *quotas.Quotas
	Navidrome navidrome.Client
	// Admin downloads the files of Attached Libraries.
	Admin    navidrome.Credentials
	Disk     common.Disk
	MusicDir string
	Clock    func() time.Time
}

func (i *ShareAlbum) Execute(ctx context.Context, trackID uint) (*ShareResult, error) {
	user, err := i.IDs.CurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	libs, err := i.Libraries.Of(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	result := &ShareResult{}
	err = libraries.Within(ctx, i.Tx, i.Lock, i.Disk, libs.ManagedLibraries, func(ctx context.Context, changes *libraries.FileChanges) error {
		usage, err := i.Quotas.UsageOf(ctx, libs.Shared)
		if err != nil {
			return err
		}
		s := &sharer{
			tracks: i.Tracks, shared: i.Shared, lock: i.Lock, disk: i.Disk,
			navidrome: i.Navidrome, admin: i.Admin, musicDir: i.MusicDir,
			user: user, libs: libs.ManagedLibraries, kept: libs.Kept(), usage: usage, changes: changes, now: i.Clock(),
		}
		track, err := keptTrack(ctx, i.Tracks, libs.Kept(), trackID)
		if err != nil {
			return err
		}
		albumTracks, err := album(ctx, i.Tracks, libs.Kept(), track)
		if err != nil {
			return err
		}
		locked, err := s.lockAttached(ctx, albumTracks)
		if err != nil {
			return err
		}
		for j := range locked {
			if err := s.share(ctx, &locked[j], result); err != nil {
				return err
			}
		}
		return nil
	})
	return result, err
}
