package import_collection

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/providers"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type PlanImport struct {
	IDs       common.IDProvider
	Providers *providers.Registry
	Libraries repositories.Libraries
	Tracks    repositories.Tracks
}

func (i *PlanImport) Execute(ctx context.Context, provider domain.ProviderName) (*Plan, error) {
	user, err := i.IDs.CurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	collection, missing, err := survey(ctx, i.Providers, i.Libraries, i.Tracks, user.ID, provider)
	if err != nil {
		return nil, err
	}
	return planOf(collection.Tracks(), missing), nil
}
