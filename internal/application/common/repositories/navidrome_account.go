package repositories

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type NavidromeAccounts interface {
	Get(ctx context.Context, userID uint) (*domain.NavidromeAccount, error)
	// ByLogin ignores case, as Navidrome logins do.
	ByLogin(ctx context.Context, login string) (*domain.NavidromeAccount, error)
	Save(ctx context.Context, account *domain.NavidromeAccount) error
}

var ErrNavidromeAccountNotFound = errors.New("navidrome account not found")
