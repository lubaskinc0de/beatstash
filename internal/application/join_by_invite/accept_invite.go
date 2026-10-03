package join_by_invite

import (
	"context"
	"log/slog"
	"time"

	"github.com/lubaskinc0de/beatstash/internal/application/common"
	"github.com/lubaskinc0de/beatstash/internal/application/common/libraries"
	"github.com/lubaskinc0de/beatstash/internal/application/common/repositories"
	"github.com/lubaskinc0de/beatstash/internal/domain/access"
	"github.com/lubaskinc0de/beatstash/internal/domain/library"
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

func (i *AcceptInvite) Execute(ctx context.Context, code string) (*access.User, error) {
	user, err := i.IDs.NewUser(ctx)
	if err != nil {
		return nil, err
	}

	var lib *library.Library
	err = i.Tx.WithinTx(ctx, func(ctx context.Context) error {
		invite, err := i.Invites.GetForUpdate(ctx, code)
		if err != nil {
			return err
		}

		if err := i.Users.Save(ctx, user); err != nil {
			return err
		}
		// A refused Invite rolls the User back.
		if err := invite.Redeem(user, i.Clock()); err != nil {
			return err
		}
		if lib, err = i.Libraries.EnsurePersonal(ctx, user); err != nil {
			return err
		}
		return i.Invites.Save(ctx, invite)
	})
	if err != nil {
		return nil, err
	}

	// Navidrome sits outside the transaction; a failure here is repaired by the next grant or start.
	if err := i.NavidromeLibraries.Create(ctx, lib); err != nil {
		slog.Error("create_personal_library_in_navidrome", "user_id", user.ID, "error", err)
	}
	return user, nil
}
