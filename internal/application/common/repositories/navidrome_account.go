package repositories

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
)

type NavidromeAccounts interface {
	Get(ctx context.Context, userID uint) (*access.NavidromeAccount, error)
	// ByLogin ignores case, as Navidrome logins do.
	ByLogin(ctx context.Context, login string) (*access.NavidromeAccount, error)
	Save(ctx context.Context, account *access.NavidromeAccount) error
}

var ErrNavidromeAccountNotFound = errors.New("navidrome account not found")
