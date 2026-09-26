package common

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
)

type IDProvider interface {
	// CurrentUser returns ErrNotAuthenticated for somebody who has not joined.
	CurrentUser(ctx context.Context) (*access.User, error)
	// NewUser describes the stranger making the request as an unsaved User.
	NewUser(ctx context.Context) (*access.User, error)
}

var ErrNotAuthenticated = errors.New("access denied")
