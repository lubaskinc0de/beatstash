package bot

import (
	"context"
	"errors"
	"log/slog"

	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type senderContextKey struct{}

type sender struct {
	from *models.User
	user *domain.User
}

func withSender(ctx context.Context, from *models.User) context.Context {
	return context.WithValue(ctx, senderContextKey{}, &sender{from: from})
}

func senderFrom(ctx context.Context) (*sender, error) {
	s, ok := ctx.Value(senderContextKey{}).(*sender)
	if !ok {
		return nil, common.ErrNotAuthenticated
	}
	return s, nil
}

type IDProvider struct {
	Users repositories.Users
}

func (p *IDProvider) CurrentUser(ctx context.Context) (*domain.User, error) {
	s, err := senderFrom(ctx)
	if err != nil {
		return nil, err
	}
	if s.user != nil {
		return s.user, nil
	}

	user, err := p.Users.GetByTelegramID(ctx, uint64(s.from.ID))
	if errors.Is(err, repositories.ErrUserNotFound) {
		return nil, common.ErrNotAuthenticated
	}
	if err != nil {
		return nil, err
	}

	// Admins from admin_ids start without a username, and people rename themselves.
	if s.from.Username != user.Username {
		if err := p.Users.SetUsername(ctx, user.ID, s.from.Username); err != nil {
			slog.Error("set_username", "error", err)
		} else {
			user.Username = s.from.Username
		}
	}
	s.user = user
	return user, nil
}

func (p *IDProvider) NewUser(ctx context.Context) (*domain.User, error) {
	s, err := senderFrom(ctx)
	if err != nil {
		return nil, err
	}
	return &domain.User{TelegramID: uint64(s.from.ID), Username: s.from.Username}, nil
}
