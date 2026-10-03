package oversee_service

import (
	"context"
	"time"

	"github.com/lubaskinc0de/beatstash/internal/application/common"
	"github.com/lubaskinc0de/beatstash/internal/application/common/quotas"
	"github.com/lubaskinc0de/beatstash/internal/application/common/repositories"
	"github.com/lubaskinc0de/beatstash/internal/domain/access"
	"github.com/lubaskinc0de/beatstash/internal/domain/library"
)

type ServiceStats struct {
	Users       int64
	ActiveUsers int64
	// Strangers wrote to the bot and never became Users.
	Strangers int64
	// Tracks counts the Tracks of all Libraries, Attached ones too.
	Tracks       int64
	SharedTracks int64
	Takes        int64
	PendingJobs  int64
	FailedJobs   int64
}

type Overview struct {
	Stats    ServiceStats
	Capacity library.Capacity
}

type UserCounter interface {
	// CountActive counts all Users and those seen since the time.
	CountActive(ctx context.Context, since time.Time) (total, active int64, err error)
}

type StrangerCounter interface {
	CountStrangers(ctx context.Context) (int64, error)
}

type TakeCounter interface {
	Count(ctx context.Context) (int64, error)
}

type JobCounter interface {
	CountByStatus(ctx context.Context) (pending, failed int64, err error)
}

type GetOverview struct {
	IDs       common.IDProvider
	Users     UserCounter
	Strangers StrangerCounter
	Libraries repositories.Libraries
	Tracks    repositories.Tracks
	Takes     TakeCounter
	Jobs      JobCounter
	Quotas    *quotas.Quotas
	Clock     func() time.Time
}

func (i *GetOverview) Execute(ctx context.Context) (*Overview, error) {
	if _, err := common.CurrentAdmin(ctx, i.IDs); err != nil {
		return nil, err
	}
	var stats ServiceStats
	var err error
	if stats.Users, stats.ActiveUsers, err = i.Users.CountActive(ctx, access.ActiveSince(i.Clock())); err != nil {
		return nil, err
	}
	if stats.Strangers, err = i.Strangers.CountStrangers(ctx); err != nil {
		return nil, err
	}
	libs, err := i.Libraries.All(ctx)
	if err != nil {
		return nil, err
	}
	if err := i.countTracks(ctx, libs, &stats); err != nil {
		return nil, err
	}
	if stats.Takes, err = i.Takes.Count(ctx); err != nil {
		return nil, err
	}
	if stats.PendingJobs, stats.FailedJobs, err = i.Jobs.CountByStatus(ctx); err != nil {
		return nil, err
	}
	_, server, err := i.Quotas.Settings(ctx)
	if err != nil {
		return nil, err
	}
	capacity, err := i.Quotas.Capacity(ctx, server, libs)
	if err != nil {
		return nil, err
	}
	return &Overview{Stats: stats, Capacity: capacity}, nil
}

func (i *GetOverview) countTracks(ctx context.Context, libs []library.Library, stats *ServiceStats) error {
	all := make([]uint, 0, len(libs))
	var shared uint
	for _, lib := range libs {
		all = append(all, lib.ID)
		if lib.Kind == library.LibraryShared {
			shared = lib.ID
		}
	}
	var err error
	if stats.Tracks, err = i.Tracks.CountIn(ctx, all); err != nil {
		return err
	}
	stats.SharedTracks, err = i.Tracks.CountIn(ctx, []uint{shared})
	return err
}
