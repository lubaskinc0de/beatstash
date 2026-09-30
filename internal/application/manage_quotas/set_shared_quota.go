package manage_quotas

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

type SetSharedQuota struct {
	IDs      common.IDProvider
	Settings repositories.QuotaSettings
}

func (i *SetSharedQuota) Execute(ctx context.Context, q *library.Quota) error {
	if _, err := common.CurrentAdmin(ctx, i.IDs); err != nil {
		return err
	}
	settings, err := i.Settings.Get(ctx)
	if err != nil {
		return err
	}
	settings.SetShared(q)
	return i.Settings.Save(ctx, settings)
}
