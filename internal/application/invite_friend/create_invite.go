package invite_friend

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"slices"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type CreateInvite struct {
	IDs      common.IDProvider
	Invites  repositories.Invites
	AdminIDs []uint64
	TTL      time.Duration
	Clock    func() time.Time
}

var ErrNotAdmin = errors.New("not an admin")

func (i *CreateInvite) Execute(ctx context.Context) (string, error) {
	user, err := i.IDs.CurrentUser(ctx)
	if err != nil {
		return "", err
	}
	if !isAdmin(i.AdminIDs, user) {
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

func isAdmin(adminIDs []uint64, user *domain.User) bool {
	return slices.Contains(adminIDs, user.TelegramID)
}
