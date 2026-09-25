package application

import (
	"context"
	"log/slog"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type userContextKey struct{}

type UserRepository interface {
	GetByTelegramID(ctx context.Context, telegramID uint64) (*domain.User, error)
	Save(ctx context.Context, user *domain.User) error
	SetUsername(ctx context.Context, userID uint, username string) error
	SetAwaitsNavidromeLogin(ctx context.Context, userID uint, awaits bool) error
}

type TelegramProfile struct {
	ID       uint64
	Username string
}

func WithUser(ctx context.Context, user *domain.User) context.Context {
	return context.WithValue(ctx, userContextKey{}, user)
}

func UserFromContext(ctx context.Context) (*domain.User, bool) {
	user, ok := ctx.Value(userContextKey{}).(*domain.User)
	return user, ok
}

func CurrentUser(ctx context.Context) (*domain.User, error) {
	user, ok := UserFromContext(ctx)
	if !ok {
		return nil, ErrNotAuthenticated
	}
	return user, nil
}

type Authenticate struct {
	Users UserRepository
}

// Execute returns ErrUserNotFound for somebody who has not joined.
func (i *Authenticate) Execute(ctx context.Context, profile TelegramProfile) (*domain.User, error) {
	user, err := i.Users.GetByTelegramID(ctx, profile.ID)
	if err != nil {
		return nil, err
	}

	// Admins from admin_ids start without a username, and people rename themselves.
	if profile.Username != user.Username {
		if err := i.Users.SetUsername(ctx, user.ID, profile.Username); err != nil {
			slog.Error("set_username", "error", err)
		} else {
			user.Username = profile.Username
		}
	}
	return user, nil
}
