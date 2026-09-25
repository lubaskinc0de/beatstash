package invite_friend

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
)

type CheckCanInvite struct {
	IDs      common.IDProvider
	AdminIDs []uint64
}

func (i *CheckCanInvite) Execute(ctx context.Context) (bool, error) {
	user, err := i.IDs.CurrentUser(ctx)
	if err != nil {
		return false, err
	}
	return isAdmin(i.AdminIDs, user), nil
}
