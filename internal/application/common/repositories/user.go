package repositories

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/beatstash/internal/domain/access"
)

type Users interface {
	GetByIdentity(ctx context.Context, identity access.Identity) (*access.User, error)
	// Identities leaves out the Users without an Identity in the Channel.
	Identities(ctx context.Context, channel access.Channel, userIDs []uint) ([]access.Identity, error)
	All(ctx context.Context) ([]access.User, error)
	Save(ctx context.Context, user *access.User) error
	// SaveSeen saves the User's profile and Last Seen only.
	SaveSeen(ctx context.Context, user *access.User) error
}

var ErrUserNotFound = errors.New("user not found")
