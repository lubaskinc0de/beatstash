package application

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/entities"
)

type UserContextKey struct{}

type TrackRepository interface {
	Save(ctx context.Context, track *entities.Track) error
	GetByUniqueId(ctx context.Context, uniqueId string) (*entities.Track, error)
	FindByTitleAndPerformer(ctx context.Context, title string, performer string) (*entities.Track, error)
}

type TxManager interface {
	WithinTx(ctx context.Context, fn func(context.Context) error) error
}

func UserFromContext(ctx context.Context) (*entities.User, bool) {
	user, ok := ctx.Value(UserContextKey{}).(*entities.User)
	return user, ok
}
