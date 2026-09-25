package connect_navidrome

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"regexp"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/accounts"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/navidrome"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

var navidromeLoginPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{3,32}$`)

// RegisterNavidromeAccount creates a Navidrome Account for a new User. When
// the login is empty, malformed or taken, the User is left awaiting another one.
type RegisterNavidromeAccount struct {
	IDs       common.IDProvider
	Navidrome navidrome.Client
	Accounts  *accounts.Navidrome
	Users     repositories.Users
	Libraries *libraries.Navidrome
	Admin     navidrome.Credentials
}

var (
	ErrNavidromeLoginInvalid = errors.New("navidrome login is not allowed")
	ErrNotAwaitingLogin      = errors.New("user does not await a navidrome login")
)

func (i *RegisterNavidromeAccount) Execute(ctx context.Context, login string) (navidrome.Credentials, error) {
	user, err := i.IDs.CurrentUser(ctx)
	if err != nil {
		return navidrome.Credentials{}, err
	}
	if !user.AwaitsNavidromeLogin {
		return navidrome.Credentials{}, ErrNotAwaitingLogin
	}

	creds, err := i.register(ctx, user, login)
	if err != nil {
		if awaitErr := i.Users.SetAwaitsNavidromeLogin(ctx, user.ID, true); awaitErr != nil {
			return navidrome.Credentials{}, errors.Join(err, awaitErr)
		}
		return navidrome.Credentials{}, err
	}

	if err := i.Users.SetAwaitsNavidromeLogin(ctx, user.ID, false); err != nil {
		return navidrome.Credentials{}, err
	}
	return creds, nil
}

func (i *RegisterNavidromeAccount) register(ctx context.Context, user *domain.User, login string) (navidrome.Credentials, error) {
	if !navidromeLoginPattern.MatchString(login) {
		return navidrome.Credentials{}, ErrNavidromeLoginInvalid
	}

	creds := navidrome.Credentials{Login: login, Password: rand.Text()}
	if err := i.Navidrome.CreateAccount(ctx, i.Admin, creds); err != nil {
		return navidrome.Credentials{}, err
	}
	if err := i.Accounts.Save(ctx, user.ID, creds); err != nil {
		return navidrome.Credentials{}, err
	}
	// The account already exists: failing here would strand it, and the next start grants again.
	if err := i.Libraries.Grant(ctx, user, creds.Login); err != nil {
		slog.Error("grant_navidrome_libraries", "user_id", user.ID, "error", err)
	}
	return creds, nil
}
