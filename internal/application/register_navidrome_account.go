package application

import (
	"context"
	"crypto/rand"
	"errors"
	"regexp"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

var ErrNavidromeLoginInvalid = errors.New("navidrome login is not allowed")

var navidromeLoginPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{3,32}$`)

// RegisterNavidromeAccount creates a Navidrome Account for a new User. When
// the login is empty, malformed or taken, the User is left awaiting another one.
type RegisterNavidromeAccount struct {
	Navidrome Navidrome
	Accounts  *NavidromeAccounts
	Users     UserRepository
	Admin     NavidromeCredentials
}

func NewRegisterNavidromeAccount(
	navidrome Navidrome,
	accounts *NavidromeAccounts,
	users UserRepository,
	admin NavidromeCredentials,
) *RegisterNavidromeAccount {
	return &RegisterNavidromeAccount{Navidrome: navidrome, Accounts: accounts, Users: users, Admin: admin}
}

func (i *RegisterNavidromeAccount) Execute(ctx context.Context, user *domain.User, login string) (NavidromeCredentials, error) {
	creds, err := i.register(ctx, user, login)
	if err != nil {
		if awaitErr := i.Users.SetAwaitsNavidromeLogin(ctx, user.ID, true); awaitErr != nil {
			return NavidromeCredentials{}, errors.Join(err, awaitErr)
		}
		return NavidromeCredentials{}, err
	}

	if err := i.Users.SetAwaitsNavidromeLogin(ctx, user.ID, false); err != nil {
		return NavidromeCredentials{}, err
	}
	return creds, nil
}

func (i *RegisterNavidromeAccount) register(ctx context.Context, user *domain.User, login string) (NavidromeCredentials, error) {
	if !navidromeLoginPattern.MatchString(login) {
		return NavidromeCredentials{}, ErrNavidromeLoginInvalid
	}

	creds := NavidromeCredentials{Login: login, Password: rand.Text()}
	if err := i.Navidrome.CreateAccount(ctx, i.Admin, creds); err != nil {
		return NavidromeCredentials{}, err
	}
	if err := i.Accounts.Save(ctx, user.ID, creds); err != nil {
		return NavidromeCredentials{}, err
	}
	return creds, nil
}
