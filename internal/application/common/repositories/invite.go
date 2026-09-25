package repositories

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type Invites interface {
	Save(ctx context.Context, invite *domain.Invite) error
	GetForUpdate(ctx context.Context, code string) (*domain.Invite, error)
}

var ErrInviteInvalid = errors.New("invite is unknown, used or expired")
