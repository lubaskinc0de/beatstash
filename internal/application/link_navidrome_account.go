package application

import "context"

type LinkNavidromeAccount struct {
	Navidrome Navidrome
	Accounts  *NavidromeAccounts
	Users     UserRepository
}

func NewLinkNavidromeAccount(navidrome Navidrome, accounts *NavidromeAccounts, users UserRepository) *LinkNavidromeAccount {
	return &LinkNavidromeAccount{Navidrome: navidrome, Accounts: accounts, Users: users}
}

func (i *LinkNavidromeAccount) Execute(ctx context.Context, creds NavidromeCredentials) error {
	user, ok := UserFromContext(ctx)
	if !ok {
		return ErrNotAuthenticated
	}

	if err := i.Navidrome.Authenticate(ctx, creds); err != nil {
		return err
	}
	if err := i.Accounts.Save(ctx, user.ID, creds); err != nil {
		return err
	}
	return i.Users.SetAwaitsNavidromeLogin(ctx, user.ID, false)
}
