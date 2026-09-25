package repositories

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type Users interface {
	GetByTelegramID(ctx context.Context, telegramID uint64) (*domain.User, error)
	Save(ctx context.Context, user *domain.User) error
	SetUsername(ctx context.Context, userID uint, username string) error
	SetAwaitsNavidromeLogin(ctx context.Context, userID uint, awaits bool) error
}

var ErrUserNotFound = errors.New("user not found")
