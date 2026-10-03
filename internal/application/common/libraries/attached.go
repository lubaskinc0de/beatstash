package libraries

import (
	"context"
	"errors"
	"log/slog"

	"github.com/lubaskinc0de/beatstash/internal/application/common/navidrome"
	"github.com/lubaskinc0de/beatstash/internal/application/common/repositories"
	"github.com/lubaskinc0de/beatstash/internal/domain/library"
)

// Attached tells which Attached Libraries a user sees. Without a Navidrome
// Account, or while Navidrome is down, a user sees none.
type Attached struct {
	Repo      repositories.Libraries
	Accounts  repositories.NavidromeAccounts
	Navidrome AccountReader
	Admin     navidrome.Credentials
}

// AccountReader may answer from a cache: a library an admin closes in
// Navidrome may stay visible for a while.
type AccountReader interface {
	// Account returns navidrome.ErrAccountNotFound if Navidrome has no such login.
	Account(ctx context.Context, admin navidrome.Credentials, login string) (*navidrome.Account, error)
}

func (a *Attached) VisibleTo(ctx context.Context, userID uint) ([]*library.Library, error) {
	all, err := a.Repo.Attached(ctx)
	if err != nil || len(all) == 0 {
		return nil, err
	}
	account, err := a.Accounts.Get(ctx, userID)
	if errors.Is(err, repositories.ErrNavidromeAccountNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	nd, err := a.Navidrome.Account(ctx, a.Admin, account.Login)
	if err != nil {
		if !errors.Is(err, navidrome.ErrAccountNotFound) {
			slog.Warn("navidrome_access", "user_id", userID, "error", err)
		}
		return nil, nil
	}
	return nd.Access.Visible(all), nil
}

func IDs(libs []*library.Library) []uint {
	ids := make([]uint, 0, len(libs))
	for _, lib := range libs {
		ids = append(ids, lib.ID)
	}
	return ids
}
