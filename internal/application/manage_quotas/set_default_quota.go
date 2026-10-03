package manage_quotas

import (
	"context"

	"github.com/lubaskinc0de/beatstash/internal/application/common"
	"github.com/lubaskinc0de/beatstash/internal/application/common/repositories"
	"github.com/lubaskinc0de/beatstash/internal/domain/library"
)

// SetDefaultQuota changes the Quota of every User without an own one.
type SetDefaultQuota struct {
	IDs      common.IDProvider
	Settings repositories.QuotaSettings
}

func (i *SetDefaultQuota) Execute(ctx context.Context, q *library.Quota) error {
	if _, err := common.CurrentAdmin(ctx, i.IDs); err != nil {
		return err
	}
	settings, err := i.Settings.Get(ctx)
	if err != nil {
		return err
	}
	settings.SetDefault(q)
	return i.Settings.Save(ctx, settings)
}
