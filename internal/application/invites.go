package application

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log/slog"
	"slices"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

var (
	ErrNotAdmin      = errors.New("not an admin")
	ErrInviteInvalid = errors.New("invite is unknown, used or expired")
)

type InviteRepository interface {
	Save(ctx context.Context, invite *domain.Invite) error
	GetForUpdate(ctx context.Context, code string) (*domain.Invite, error)
}

type CreateInvite struct {
	Invites  InviteRepository
	AdminIDs []uint64
	TTL      time.Duration
	Clock    func() time.Time
}

func (i *CreateInvite) Execute(ctx context.Context) (string, error) {
	user, err := CurrentUser(ctx)
	if err != nil {
		return "", err
	}
	if !i.CanInvite(user) {
		return "", ErrNotAdmin
	}

	code := make([]byte, 16)
	if _, err := rand.Read(code); err != nil {
		return "", err
	}

	now := i.Clock()
	invite := &domain.Invite{
		Code:      base64.RawURLEncoding.EncodeToString(code),
		CreatedBy: user.ID,
		CreatedAt: now,
		ExpiresAt: now.Add(i.TTL),
	}
	if err := i.Invites.Save(ctx, invite); err != nil {
		return "", err
	}
	return invite.Code, nil
}

func (i *CreateInvite) CanInvite(user *domain.User) bool {
	return slices.Contains(i.AdminIDs, user.TelegramID)
}

type AcceptInvite struct {
	Tx        TxManager
	Invites   InviteRepository
	Users     UserRepository
	Libraries *Libraries
	Clock     func() time.Time
}

func (i *AcceptInvite) Execute(ctx context.Context, code string, profile TelegramProfile) (*domain.User, error) {
	var (
		user    *domain.User
		library *domain.Library
	)
	err := i.Tx.WithinTx(ctx, func(ctx context.Context) error {
		invite, err := i.Invites.GetForUpdate(ctx, code)
		if err != nil {
			return err
		}

		now := i.Clock()
		if !invite.Redeemable(now) {
			return ErrInviteInvalid
		}

		// Awaiting from the start: a crash before the account exists must not strand the User.
		user = &domain.User{TelegramID: profile.ID, Username: profile.Username, CreatedAt: now, AwaitsNavidromeLogin: true}
		if err := i.Users.Save(ctx, user); err != nil {
			return err
		}
		if library, err = i.Libraries.Personal(ctx, user); err != nil {
			return err
		}

		invite.UsedBy = &user.ID
		invite.UsedAt = &now
		return i.Invites.Save(ctx, invite)
	})
	if err != nil {
		return nil, err
	}

	// Navidrome sits outside the transaction; a failure here is repaired by the next grant or start.
	if err := i.Libraries.CreateInNavidrome(ctx, library); err != nil {
		slog.Error("create_personal_library_in_navidrome", "user_id", user.ID, "error", err)
	}
	return user, nil
}
