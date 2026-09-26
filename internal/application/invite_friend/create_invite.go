package invite_friend

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
)

type CreateInvite struct {
	IDs     common.IDProvider
	Invites repositories.Invites
	TTL     time.Duration
	Clock   func() time.Time
}

func (i *CreateInvite) Execute(ctx context.Context) (string, error) {
	user, err := i.IDs.CurrentUser(ctx)
	if err != nil {
		return "", err
	}

	code := make([]byte, 16)
	if _, err := rand.Read(code); err != nil {
		return "", err
	}
	invite, err := access.NewInvite(base64.RawURLEncoding.EncodeToString(code), user, i.Clock(), i.TTL)
	if err != nil {
		return "", err
	}
	if err := i.Invites.Save(ctx, invite); err != nil {
		return "", err
	}
	return invite.Code, nil
}
