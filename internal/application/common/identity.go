package common

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
)

type IDProvider interface {
	// CurrentUser returns ErrNotAuthenticated for somebody who has not joined.
	CurrentUser(ctx context.Context) (*access.User, error)
	// NewUser describes the stranger making the request as an unsaved User.
	NewUser(ctx context.Context) (*access.User, error)
}

var ErrNotAuthenticated = errors.New("access denied")

// Requesters is what a Channel tells of who makes the request.
type Requesters interface {
	IDProvider
	// Profile is false when the service acts for the User rather than the
	// User asking, e.g. to deliver a notice.
	Profile(ctx context.Context) (access.Profile, bool)
}

// Visitors is the IDProvider of the interactors: finding the User who
// asks also counts the request, so Last Seen and the profile follow every
// request, inline queries in other chats too.
type Visitors struct {
	Channel Requesters
	Users   repositories.Users
	Clock   func() time.Time
}

// CurrentUser only logs a failed save of Last Seen: the request goes on.
func (v *Visitors) CurrentUser(ctx context.Context) (*access.User, error) {
	user, err := v.Channel.CurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	if profile, asked := v.Channel.Profile(ctx); asked && user.Seen(profile, v.Clock()) {
		if err := v.Users.SaveSeen(ctx, user); err != nil {
			slog.Error("save_last_seen", "user_id", user.ID, "error", err)
		}
	}
	return user, nil
}

func (v *Visitors) NewUser(ctx context.Context) (*access.User, error) {
	return v.Channel.NewUser(ctx)
}

func CurrentAdmin(ctx context.Context, ids IDProvider) (*access.User, error) {
	user, err := ids.CurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := user.RequireAdmin(); err != nil {
		return nil, err
	}
	return user, nil
}
