package bot

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
)

type senderContextKey struct{}

type sender struct {
	from *models.User
	user *access.User
	// recipient: the Poller acts for the user; from has only the id.
	recipient bool
}

func withSender(ctx context.Context, from *models.User) context.Context {
	return context.WithValue(ctx, senderContextKey{}, &sender{from: from})
}

// asRecipient lets the Poller run interactors as the user it writes to.
func asRecipient(ctx context.Context, telegramID int64) context.Context {
	return context.WithValue(ctx, senderContextKey{}, &sender{from: &models.User{ID: telegramID}, recipient: true})
}

func senderFrom(ctx context.Context) (*sender, error) {
	s, ok := ctx.Value(senderContextKey{}).(*sender)
	if !ok {
		return nil, common.ErrNotAuthenticated
	}
	return s, nil
}

const Channel access.Channel = "telegram"

func Identity(telegramID int64) access.Identity {
	return access.Identity{Channel: Channel, ExternalID: strconv.FormatInt(telegramID, 10)}
}

type IDProvider struct {
	Users repositories.Users
	Clock func() time.Time
}

func (p *IDProvider) CurrentUser(ctx context.Context) (*access.User, error) {
	s, err := senderFrom(ctx)
	if err != nil {
		return nil, err
	}
	if s.user != nil {
		return s.user, nil
	}

	user, err := p.Users.GetByIdentity(ctx, Identity(s.from.ID))
	if errors.Is(err, repositories.ErrUserNotFound) {
		return nil, common.ErrNotAuthenticated
	}
	if err != nil {
		return nil, err
	}

	// Admins from the config start without a username, and people rename themselves.
	if !s.recipient && s.from.Username != user.Username {
		if err := p.Users.SetUsername(ctx, user.ID, s.from.Username); err != nil {
			slog.Error("set_username", "error", err)
		} else {
			user.Rename(s.from.Username)
		}
	}
	s.user = user
	return user, nil
}

func (p *IDProvider) NewUser(ctx context.Context) (*access.User, error) {
	s, err := senderFrom(ctx)
	if err != nil {
		return nil, err
	}
	return access.NewUser(s.from.Username, Identity(s.from.ID), p.Clock()), nil
}
