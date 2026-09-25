package application

import (
	"context"
	"errors"
)

type LinkNavidromeAccount struct {
	Navidrome Navidrome
	Accounts  *NavidromeAccounts
	Users     UserRepository
	Libraries *Libraries
}

func (i *LinkNavidromeAccount) Execute(ctx context.Context, creds NavidromeCredentials) error {
	user, err := CurrentUser(ctx)
	if err != nil {
		return err
	}

	if err := i.Navidrome.Authenticate(ctx, creds); err != nil {
		return err
	}
	if err := i.Accounts.CheckFree(ctx, user.ID, creds.Login); err != nil {
		return err
	}
	// Access narrows before the account counts as linked: a failure must not
	// leave a linked account that still sees others' Personal Libraries.
	granted := i.Libraries.Grant(ctx, user, creds.Login)
	if granted != nil && !errors.Is(granted, ErrNavidromeAdminAccount) {
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
