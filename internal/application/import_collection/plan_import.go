package import_collection

import (
	"context"

	"github.com/lubaskinc0de/beatstash/internal/application/common"
	"github.com/lubaskinc0de/beatstash/internal/application/common/libraries"
	"github.com/lubaskinc0de/beatstash/internal/application/common/providers"
	"github.com/lubaskinc0de/beatstash/internal/application/common/quotas"
	"github.com/lubaskinc0de/beatstash/internal/application/common/repositories"
	"github.com/lubaskinc0de/beatstash/internal/domain/provider"
)

type PlanImport struct {
	IDs       common.IDProvider
	Providers *providers.Registry
	Libraries *libraries.Libraries
	Tracks    repositories.Tracks
	Quotas    *quotas.Quotas
}

func (i *PlanImport) Execute(ctx context.Context, providerName provider.ProviderName) (*Plan, error) {
	user, err := i.IDs.CurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	surveyed, err := survey(ctx, i.Providers, i.Libraries, i.Tracks, user.ID, providerName)
	if err != nil {
		return nil, err
	}
	plan := surveyed.plan()
	plan.Usage, err = i.Quotas.UsageOf(ctx, surveyed.personal)
	return plan, err
}
