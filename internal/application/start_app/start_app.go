package start_app

import (
	"context"
	"errors"
	"log/slog"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/navidrome"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

// StartApp creates Admins first so they get Personal Libraries. Navidrome
// being down is not fatal: the next start or grant finishes the job.
type StartApp struct {
	Admins             AdminRegistry
	AdminIDs           []uint64
	Users              UserLister
	Accounts           NavidromeAccountLister
	Libraries          *libraries.Libraries
	NavidromeLibraries *libraries.Navidrome
}

type AdminRegistry interface {
	EnsureExist(ctx context.Context, telegramIDs []uint64) error
}

type UserLister interface {
	All(ctx context.Context) ([]domain.User, error)
}

type NavidromeAccountLister interface {
	All(ctx context.Context) ([]domain.NavidromeAccount, error)
}

func (i *StartApp) Execute(ctx context.Context) error {
	if err := i.Admins.EnsureExist(ctx, i.AdminIDs); err != nil {
		return err
	}
	if _, err := i.Libraries.Shared(ctx); err != nil {
		return err
	}
	users, err := i.Users.All(ctx)
	if err != nil {
		return err
	}
	byID := make(map[uint]*domain.User, len(users))
	for n := range users {
		byID[users[n].ID] = &users[n]
		if _, err := i.Libraries.Personal(ctx, &users[n]); err != nil {
			return err
		}
	}

	accounts, err := i.Accounts.All(ctx)
	if err != nil {
		return err
	}
	for _, account := range accounts {
		err := i.NavidromeLibraries.Grant(ctx, byID[account.UserID], account.Login)
		if err != nil && !errors.Is(err, navidrome.ErrAdminAccount) {
			slog.Error("grant_navidrome_libraries", "user_id", account.UserID, "error", err)
		}
	}

	libraries, err := i.Libraries.Repo.All(ctx)
	if err != nil {
		return err
	}
	for n := range libraries {
		if err := i.NavidromeLibraries.Create(ctx, &libraries[n]); err != nil {
			slog.Error("create_library_in_navidrome", "dir", libraries[n].Dir, "error", err)
		}
	}
	if err := i.NavidromeLibraries.ShowNewAccountsOnlyShared(ctx); err != nil {
		slog.Error("set_navidrome_default_libraries", "error", err)
	}
	return nil
}
