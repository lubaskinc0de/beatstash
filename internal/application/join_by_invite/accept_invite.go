package join_by_invite

import (
	"context"
	"log/slog"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type AcceptInvite struct {
	IDs                common.IDProvider
	Tx                 repositories.TxManager
	Invites            repositories.Invites
	Users              repositories.Users
	Libraries          *libraries.Libraries
	NavidromeLibraries *libraries.Navidrome
	Clock              func() time.Time
}

func (i *AcceptInvite) Execute(ctx context.Context, code string) (*domain.User, error) {
	user, err := i.IDs.NewUser(ctx)
	if err != nil {
		return nil, err
	}

	var library *domain.Library
	err = i.Tx.WithinTx(ctx, func(ctx context.Context) error {
		invite, err := i.Invites.GetForUpdate(ctx, code)
		if err != nil {
			return err
		}

		now := i.Clock()
		if !invite.Redeemable(now) {
			return repositories.ErrInviteInvalid
		}

		// Awaiting from the start: a crash before the account exists must not strand the User.
		user.CreatedAt = now
		user.AwaitsNavidromeLogin = true
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
	if err := i.NavidromeLibraries.Create(ctx, library); err != nil {
		slog.Error("create_personal_library_in_navidrome", "user_id", user.ID, "error", err)
	}
	return user, nil
}
