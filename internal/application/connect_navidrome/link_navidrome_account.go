package connect_navidrome

import (
	"context"
	"errors"
	"log/slog"

	"github.com/lubaskinc0de/beatstash/internal/application/common"
	"github.com/lubaskinc0de/beatstash/internal/application/common/accounts"
	"github.com/lubaskinc0de/beatstash/internal/application/common/libraries"
	"github.com/lubaskinc0de/beatstash/internal/application/common/navidrome"
	"github.com/lubaskinc0de/beatstash/internal/application/common/repositories"
	"github.com/lubaskinc0de/beatstash/internal/domain/library"
)

type LinkNavidromeAccount struct {
	IDs                common.IDProvider
	Tx                 repositories.TxManager
	Navidrome          navidrome.Client
	Accounts           *accounts.Navidrome
	NavidromeLibraries *libraries.Navidrome
	Libraries          repositories.Libraries
	Tracks             repositories.Tracks
}

var ErrNavidromeAccountTaken = errors.New("navidrome account is linked to another user")

// Execute returns how many songs of Attached Libraries the account lets the
// user see.
func (i *LinkNavidromeAccount) Execute(ctx context.Context, creds navidrome.Credentials) (songs int, err error) {
	user, err := i.IDs.CurrentUser(ctx)
	if err != nil {
		return 0, err
	}

	if err := i.Navidrome.Authenticate(ctx, creds); err != nil {
		return 0, err
	}
	var access library.NavidromeAccess
	var granted error
	// The account is written first: another user linking the login at the
	// same time waits on it and is refused before their Grant could narrow
	// the account to their library. It counts as linked only at the commit,
	// once access has narrowed: a failure must not leave a linked account
	// that still sees others' Personal Libraries.
	err = i.Tx.WithinTx(ctx, func(ctx context.Context) error {
		err := i.Accounts.Save(ctx, user.ID, creds)
		if errors.Is(err, repositories.ErrNavidromeLoginLinked) {
			return ErrNavidromeAccountTaken
		}
		if err != nil {
			return err
		}
		access, granted = i.NavidromeLibraries.Grant(ctx, user, creds.Login)
		if granted != nil && !errors.Is(granted, navidrome.ErrAdminAccount) {
			return granted
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return i.attachedSongCount(ctx, access), granted
}

// attachedSongCount only logs a failure: the account is linked by then.
func (i *LinkNavidromeAccount) attachedSongCount(ctx context.Context, access library.NavidromeAccess) int {
	attached, err := i.Libraries.Attached(ctx)
	if err != nil {
		slog.Error("list_attached_libraries", "error", err)
		return 0
	}
	visible := access.Visible(attached)
	if len(visible) == 0 {
		return 0
	}
	n, err := i.Tracks.CountIn(ctx, libraries.IDs(visible))
	if err != nil {
		slog.Error("count_attached_tracks", "error", err)
		return 0
	}
	return int(n)
}
