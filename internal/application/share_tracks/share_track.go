package share_tracks

import (
	"context"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/navidrome"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/quotas"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

type ShareTrack struct {
	IDs       common.IDProvider
	Tx        repositories.TxManager
	Lock      repositories.LibraryLock
	Tracks    repositories.Tracks
	Shared    repositories.SharedTracks
	Libraries *libraries.Libraries
	Attached  *libraries.Attached
	Quotas    *quotas.Quotas
	Navidrome navidrome.Client
	// Admin downloads the files of Attached Libraries.
	Admin    navidrome.Credentials
	Disk     common.Disk
	MusicDir string
	Clock    func() time.Time
}

func (i *ShareTrack) Execute(ctx context.Context, trackID uint) (*ShareResult, error) {
	result := &ShareResult{}
	s := &sharer{
		tracks: i.Tracks, shared: i.Shared, lock: i.Lock, disk: i.Disk,
		navidrome: i.Navidrome, admin: i.Admin, musicDir: i.MusicDir,
	}
	err := s.within(ctx, i.IDs, i.Libraries, i.Attached, i.Tx, i.Quotas, i.Clock(), func(ctx context.Context) error {
		track, err := keptTrack(ctx, i.Tracks, s.kept, trackID)
		if err != nil {
			return err
		}
		locked, err := s.lockAttached(ctx, []library.Track{*track})
		if err != nil {
			return err
		}
		for j := range locked {
			if err := s.share(ctx, &locked[j], result); err != nil {
				return err
			}
		}
		albumTracks, err := album(ctx, i.Tracks, s.kept, track)
		if err != nil {
			return err
		}
		result.State, err = shareState(ctx, i.Shared, track, albumTracks)
		return err
	})
	return result, err
}
