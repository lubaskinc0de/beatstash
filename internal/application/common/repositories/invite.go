package repositories

import (
	"context"

	"github.com/lubaskinc0de/beatstash/internal/domain/access"
)

type Invites interface {
	Save(ctx context.Context, invite *access.Invite) error
	// GetForUpdate returns access.ErrInviteInvalid for an unknown code.
	GetForUpdate(ctx context.Context, code string) (*access.Invite, error)
}
