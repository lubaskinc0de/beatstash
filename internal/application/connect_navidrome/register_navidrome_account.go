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
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
)

var navidromeLoginPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{3,32}$`)

// RegisterNavidromeAccount creates a Navidrome Account for a User who has none.
type RegisterNavidromeAccount struct {
	IDs       common.IDProvider
	Navidrome navidrome.Client
	Accounts  *accounts.Navidrome
	Libraries *libraries.Navidrome
	Admin     navidrome.Credentials
}

var (
	ErrNavidromeLoginInvalid = errors.New("navidrome login is not allowed")
	ErrHasNavidromeAccount   = errors.New("user has a navidrome account already")
)

func (i *RegisterNavidromeAccount) Execute(ctx context.Context, login string) (navidrome.Credentials, error) {
	user, err := i.IDs.CurrentUser(ctx)
	if err != nil {
		return navidrome.Credentials{}, err
	}
	_, err = i.Accounts.Repo.Get(ctx, user.ID)
	switch {
	case err == nil:
		return navidrome.Credentials{}, ErrHasNavidromeAccount
	case !errors.Is(err, repositories.ErrNavidromeAccountNotFound):
		return navidrome.Credentials{}, err
	}
	return i.register(ctx, user, login)
}

func (i *RegisterNavidromeAccount) register(ctx context.Context, user *access.User, login string) (navidrome.Credentials, error) {
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
	if _, err := i.Libraries.Grant(ctx, user, creds.Login); err != nil {
		slog.Error("grant_navidrome_libraries", "user_id", user.ID, "error", err)
	}
	return creds, nil
}
