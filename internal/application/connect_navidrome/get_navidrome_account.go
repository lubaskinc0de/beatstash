package connect_navidrome

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
)

type GetNavidromeAccount struct {
	IDs      common.IDProvider
	Accounts repositories.NavidromeAccounts
}

// Execute returns nil if the User has no Navidrome Account.
func (i *GetNavidromeAccount) Execute(ctx context.Context) (*access.NavidromeAccount, error) {
	user, err := i.IDs.CurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	account, err := i.Accounts.Get(ctx, user.ID)
	if errors.Is(err, repositories.ErrNavidromeAccountNotFound) {
		return nil, nil
	}
	return account, err
}
