package view_home

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/quotas"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

type Home struct {
	Admin bool
	// Usage is the Personal Library's.
	Usage library.Usage
}

type GetHome struct {
	IDs    common.IDProvider
	Quotas *quotas.Quotas
}

func (i *GetHome) Execute(ctx context.Context) (*Home, error) {
	user, err := i.IDs.CurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	usage, err := i.Quotas.PersonalUsage(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	return &Home{Admin: user.Admin, Usage: usage}, nil
}
