package repositories

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/beatstash/internal/domain/access"
)

type NavidromeAccounts interface {
	Get(ctx context.Context, userID uint) (*access.NavidromeAccount, error)
	// Add returns ErrNavidromeAccountExists or ErrNavidromeLoginLinked.
	Add(ctx context.Context, account *access.NavidromeAccount) error
	// Save replaces the user's account; it returns ErrNavidromeLoginLinked.
	Save(ctx context.Context, account *access.NavidromeAccount) error
}

var (
	ErrNavidromeAccountNotFound = errors.New("navidrome account not found")
	ErrNavidromeAccountExists   = errors.New("user has a navidrome account already")
	ErrNavidromeLoginLinked     = errors.New("navidrome login is linked to another user")
)
