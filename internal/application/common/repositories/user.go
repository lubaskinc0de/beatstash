package repositories

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
)

type Users interface {
	GetByIdentity(ctx context.Context, identity access.Identity) (*access.User, error)
	All(ctx context.Context) ([]access.User, error)
	Save(ctx context.Context, user *access.User) error
	SetUsername(ctx context.Context, userID uint, username string) error
}

var ErrUserNotFound = errors.New("user not found")
