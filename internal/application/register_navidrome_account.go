package application

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
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
	Libraries *Libraries
	Admin     NavidromeCredentials
}

func NewRegisterNavidromeAccount(
	navidrome Navidrome,
	accounts *NavidromeAccounts,
	users UserRepository,
	libraries *Libraries,
	admin NavidromeCredentials,
) *RegisterNavidromeAccount {
	return &RegisterNavidromeAccount{Navidrome: navidrome, Accounts: accounts, Users: users, Libraries: libraries, Admin: admin}
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
	// The account already exists: failing here would strand it, and the next start grants again.
	if err := i.Libraries.Grant(ctx, user, creds.Login); err != nil {
		slog.Error("grant_navidrome_libraries", "user_id", user.ID, "error", err)
	}
	return creds, nil
}
