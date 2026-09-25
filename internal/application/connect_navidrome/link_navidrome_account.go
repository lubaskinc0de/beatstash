package connect_navidrome

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/accounts"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/navidrome"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
)

type LinkNavidromeAccount struct {
	IDs       common.IDProvider
	Navidrome navidrome.Client
	Accounts  *accounts.Navidrome
	Linked    repositories.NavidromeAccounts
	Users     repositories.Users
	Libraries *libraries.Navidrome
}

var ErrNavidromeAccountTaken = errors.New("navidrome account is linked to another user")

func (i *LinkNavidromeAccount) Execute(ctx context.Context, creds navidrome.Credentials) error {
	user, err := i.IDs.CurrentUser(ctx)
	if err != nil {
		return err
	}

	if err := i.Navidrome.Authenticate(ctx, creds); err != nil {
		return err
	}
	if err := i.checkFree(ctx, user.ID, creds.Login); err != nil {
		return err
	}
	// Access narrows before the account counts as linked: a failure must not
	// leave a linked account that still sees others' Personal Libraries.
	granted := i.Libraries.Grant(ctx, user, creds.Login)
	if granted != nil && !errors.Is(granted, navidrome.ErrAdminAccount) {
		return granted
	}
	if err := i.Accounts.Save(ctx, user.ID, creds); err != nil {
		return err
	}
	if err := i.Users.SetAwaitsNavidromeLogin(ctx, user.ID, false); err != nil {
		return err
	}
	return granted
}

func (i *LinkNavidromeAccount) checkFree(ctx context.Context, userID uint, login string) error {
	linked, err := i.Linked.ByLogin(ctx, login)
	switch {
	case errors.Is(err, repositories.ErrNavidromeAccountNotFound):
		return nil
	case err != nil:
		return err
	case linked.UserID != userID:
		return ErrNavidromeAccountTaken
	}
	return nil
}
