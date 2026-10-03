package connect_navidrome

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"regexp"

	"github.com/lubaskinc0de/beatstash/internal/application/common"
	"github.com/lubaskinc0de/beatstash/internal/application/common/accounts"
	"github.com/lubaskinc0de/beatstash/internal/application/common/libraries"
	"github.com/lubaskinc0de/beatstash/internal/application/common/navidrome"
	"github.com/lubaskinc0de/beatstash/internal/application/common/repositories"
)

var navidromeLoginPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{3,32}$`)

type RegisterNavidromeAccount struct {
	IDs                common.IDProvider
	Tx                 repositories.TxManager
	Navidrome          navidrome.Client
	Accounts           *accounts.Navidrome
	NavidromeLibraries *libraries.Navidrome
	Admin              navidrome.Credentials
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
	if !navidromeLoginPattern.MatchString(login) {
		return navidrome.Credentials{}, ErrNavidromeLoginInvalid
	}

	creds := navidrome.Credentials{Login: login, Password: rand.Text()}
	// The account is written first: a second login sent at the same time
	// waits on it and finds the account there, instead of making another one
	// in Navidrome.
	err = i.Tx.WithinTx(ctx, func(ctx context.Context) error {
		err := i.Accounts.Add(ctx, user.ID, creds)
		switch {
		case errors.Is(err, repositories.ErrNavidromeAccountExists):
			return ErrHasNavidromeAccount
		case errors.Is(err, repositories.ErrNavidromeLoginLinked):
			return navidrome.ErrLoginTaken
		case err != nil:
			return err
		}
		return i.Navidrome.CreateAccount(ctx, i.Admin, creds)
	})
	if err != nil {
		return navidrome.Credentials{}, err
	}
	// The account already exists: failing here would strand it, and the next start grants again.
	if _, err := i.NavidromeLibraries.Grant(ctx, user, creds.Login); err != nil {
		slog.Error("grant_navidrome_libraries", "user_id", user.ID, "error", err)
	}
	return creds, nil
}
