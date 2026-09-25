package common

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type IDProvider interface {
	// CurrentUser returns ErrNotAuthenticated for somebody who has not joined.
	CurrentUser(ctx context.Context) (*domain.User, error)
	// NewUser describes the stranger making the request as an unsaved User.
	NewUser(ctx context.Context) (*domain.User, error)
}

var ErrNotAuthenticated = errors.New("access denied")
