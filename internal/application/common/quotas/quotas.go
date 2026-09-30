package quotas

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

// Quotas tells how much of its Quota a Library takes. Hold the Library's
// lock while a Track is admitted: two Tracks admitted at once would
// exceed it together.
type Quotas struct {
	Repo      repositories.QuotaSettings
	Libraries repositories.Libraries
	Tracks    repositories.Tracks
	Disk      common.Disk
	Config    library.ServerQuotas
}

// Settings returns what the Admin set and the server's Quotas that hold
// with it.
func (q *Quotas) Settings(ctx context.Context) (*library.QuotaSettings, library.ServerQuotas, error) {
	settings, err := q.Repo.Get(ctx)
	if err != nil {
		return nil, library.ServerQuotas{}, err
	}
	return settings, settings.Apply(q.Config), nil
}

func (q *Quotas) UsageOf(ctx context.Context, lib *library.Library) (library.Usage, error) {
	_, server, err := q.Settings(ctx)
	if err != nil {
		return library.Usage{}, err
	}
	used, err := q.Tracks.Weigh(ctx, []uint{lib.ID})
	if err != nil {
		return library.Usage{}, err
	}
	return lib.Usage(used[lib.ID], server), nil
}

func (q *Quotas) PersonalUsage(ctx context.Context, userID uint) (library.Usage, error) {
	personal, err := q.Libraries.Personal(ctx, userID)
	if err != nil {
		return library.Usage{}, err
	}
	return q.UsageOf(ctx, personal)
}

// Admit returns a *library.QuotaExceededError unless the Library can grow
// by growth bytes.
func (q *Quotas) Admit(ctx context.Context, lib *library.Library, growth int64) error {
	usage, err := q.UsageOf(ctx, lib)
	if err != nil {
		return err
	}
	return usage.Admit(growth)
}

// Capacity weighs the libraries, all the server has, and asks the disk.
func (q *Quotas) Capacity(ctx context.Context, server library.ServerQuotas, libs []library.Library) (library.Capacity, error) {
	ids := make([]uint, 0, len(libs))
	for _, lib := range libs {
		ids = append(ids, lib.ID)
	}
	weights, err := q.Tracks.Weigh(ctx, ids)
	if err != nil {
		return library.Capacity{}, err
	}
	free, err := q.Disk.FreeSpace()
	if err != nil {
		return library.Capacity{}, err
	}
	return library.CapacityOf(libs, server, weights, free), nil
}
