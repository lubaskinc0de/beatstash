package application

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/entities"
)

type UserContextKey struct{}

type TxManager interface {
	WithinTx(ctx context.Context, fn func(context.Context) error) error
}

func UserFromContext(ctx context.Context) (*entities.User, bool) {
	user, ok := ctx.Value(UserContextKey{}).(*entities.User)
	return user, ok
}
