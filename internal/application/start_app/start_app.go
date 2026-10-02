package start_app

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/navidrome"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
)

// StartApp creates Admins first so they get Personal Libraries. Navidrome
// being down is not fatal: the next start or grant finishes the job.
type StartApp struct {
	// Admins are the Identities the config names; everybody else loses Admin.
	Admins             []access.Identity
	Users              repositories.Users
	Accounts           NavidromeAccountLister
	Libraries          *libraries.Libraries
	NavidromeLibraries *libraries.Navidrome
	Clock              func() time.Time
}

type NavidromeAccountLister interface {
	All(ctx context.Context) ([]access.NavidromeAccount, error)
}

func (i *StartApp) Execute(ctx context.Context) error {
	if err := i.appointAdmins(ctx); err != nil {
		return err
	}
	if _, err := i.Libraries.EnsureShared(ctx); err != nil {
		return err
	}
	users, err := i.Users.All(ctx)
	if err != nil {
		return err
	}
	byID := make(map[uint]*access.User, len(users))
	for n := range users {
		byID[users[n].ID] = &users[n]
		if _, err := i.Libraries.EnsurePersonal(ctx, &users[n]); err != nil {
			return err
		}
	}

	accounts, err := i.Accounts.All(ctx)
	if err != nil {
		return err
	}
	for _, account := range accounts {
		_, err := i.NavidromeLibraries.Grant(ctx, byID[account.UserID], account.Login)
		if err != nil && !errors.Is(err, navidrome.ErrAdminAccount) {
			slog.Error("grant_navidrome_libraries", "user_id", account.UserID, "error", err)
		}
	}

	libraries, err := i.Libraries.Repo.All(ctx)
	if err != nil {
		return err
	}
	for n := range libraries {
		if libraries[n].Attached() {
			continue
		}
		if err := i.NavidromeLibraries.Create(ctx, &libraries[n]); err != nil {
			slog.Error("create_library_in_navidrome", "dir", libraries[n].Dir, "error", err)
		}
	}
	if err := i.NavidromeLibraries.ShowNewAccountsOnlyShared(ctx); err != nil {
		slog.Error("set_navidrome_default_libraries", "error", err)
	}
	return nil
}

// appointAdmins creates the Admins nobody has invited, so they can invite others.
func (i *StartApp) appointAdmins(ctx context.Context) error {
	admins := map[uint]bool{}
	for _, identity := range i.Admins {
		user, err := i.Users.GetByIdentity(ctx, identity)
		if errors.Is(err, repositories.ErrUserNotFound) {
			user = access.NewUser(access.Profile{}, identity, i.Clock())
			err = i.Users.Save(ctx, user)
		}
		if err != nil {
			return err
		}
		admins[user.ID] = true
	}

	users, err := i.Users.All(ctx)
	if err != nil {
		return err
	}
	for n := range users {
		user := &users[n]
		if user.Admin == admins[user.ID] {
			continue
		}
		user.SetAdmin(admins[user.ID])
		if err := i.Users.Save(ctx, user); err != nil {
			return err
		}
	}
	return nil
}
