package application

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type UserRepository interface {
	GetByTelegramID(ctx context.Context, telegramID uint64) (*domain.User, error)
	Save(ctx context.Context, user *domain.User) error
	SetUsername(ctx context.Context, userID uint, username string) error
	SetAwaitsNavidromeLogin(ctx context.Context, userID uint, awaits bool) error
}

type IDProvider interface {
	// CurrentUser returns ErrNotAuthenticated for somebody who has not joined.
	CurrentUser(ctx context.Context) (*domain.User, error)
	// NewUser describes the stranger making the request as an unsaved User.
	NewUser(ctx context.Context) (*domain.User, error)
}
