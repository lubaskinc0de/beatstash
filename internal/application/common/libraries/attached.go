package libraries

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/navidrome"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

// Attached tells which Attached Libraries a user sees. Without a Navidrome
// Account, or while Navidrome is down, a user sees none.
type Attached struct {
	Repo      repositories.Libraries
	Accounts  repositories.NavidromeAccounts
	Navidrome navidrome.Client
	Admin     navidrome.Credentials

	mu     sync.Mutex
	access map[string]cachedAccess
}

// accessTTL spares Navidrome a request per Ingest job of an Import; a
// library an admin closes stays visible this long at most.
const accessTTL = 30 * time.Second

type cachedAccess struct {
	access library.NavidromeAccess
	until  time.Time
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
	access, err := a.accessOf(ctx, account.Login)
	if err != nil {
		if !errors.Is(err, navidrome.ErrAccountNotFound) {
			slog.Warn("navidrome_access", "user_id", userID, "error", err)
		}
		return nil, nil
	}
	return access.Visible(all), nil
}

func (a *Attached) accessOf(ctx context.Context, login string) (library.NavidromeAccess, error) {
	a.mu.Lock()
	cached, ok := a.access[login]
	a.mu.Unlock()
	if ok && time.Now().Before(cached.until) {
		return cached.access, nil
	}
	account, err := a.Navidrome.Account(ctx, a.Admin, login)
	if err != nil {
		return library.NavidromeAccess{}, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.access == nil {
		a.access = map[string]cachedAccess{}
	}
	a.access[login] = cachedAccess{access: account.Access, until: time.Now().Add(accessTTL)}
	return account.Access, nil
}

func IDs(libs []*library.Library) []uint {
	ids := make([]uint, 0, len(libs))
	for _, lib := range libs {
		ids = append(ids, lib.ID)
	}
	return ids
}
