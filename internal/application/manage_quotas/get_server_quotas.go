package manage_quotas

import (
	"context"

	"github.com/lubaskinc0de/beatstash/internal/application/common"
	"github.com/lubaskinc0de/beatstash/internal/application/common/quotas"
	"github.com/lubaskinc0de/beatstash/internal/application/common/repositories"
	"github.com/lubaskinc0de/beatstash/internal/domain/library"
)

type ServerQuotas struct {
	Default  ServerQuota
	Shared   ServerQuota
	Capacity library.Capacity
}

type ServerQuota struct {
	Quota library.Quota
	// FromConfig is true until the Admin sets the Quota.
	FromConfig bool
}

type GetServerQuotas struct {
	IDs       common.IDProvider
	Libraries repositories.Libraries
	Quotas    *quotas.Quotas
}

func (i *GetServerQuotas) Execute(ctx context.Context) (*ServerQuotas, error) {
	if _, err := common.CurrentAdmin(ctx, i.IDs); err != nil {
		return nil, err
	}
	settings, server, err := i.Quotas.Settings(ctx)
	if err != nil {
		return nil, err
	}
	q := ServerQuotas{
		Default: ServerQuota{Quota: server.Default, FromConfig: settings.DefaultFromConfig()},
		Shared:  ServerQuota{Quota: server.Shared, FromConfig: settings.SharedFromConfig()},
	}
	libs, err := i.Libraries.All(ctx)
	if err != nil {
		return nil, err
	}
	if q.Capacity, err = i.Quotas.Capacity(ctx, server, libs); err != nil {
		return nil, err
	}
	return &q, nil
}
