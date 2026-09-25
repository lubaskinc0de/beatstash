package browse_shared

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
)

type SendFile struct {
	IDs       common.IDProvider
	Tx        repositories.TxManager
	Tracks    repositories.Tracks
	Libraries *libraries.Libraries
	Sender    common.AudioSender
}

func (i *SendFile) Execute(ctx context.Context, sharedTrackID uint, chatID int64) error {
	if _, err := i.IDs.CurrentUser(ctx); err != nil {
		return err
	}
	shared, err := i.Libraries.Shared(ctx)
	if err != nil {
		return err
	}
	track, err := sharedTrack(ctx, i.Tracks, shared, sharedTrackID)
	if err != nil {
		return err
	}

	file, err := i.Tracks.TelegramFile(ctx, track.ID)
	if err == nil {
		return i.Sender.Send(ctx, chatID, file)
	}
	if !errors.Is(err, repositories.ErrNoTelegramFile) {
		return err
	}
	path, err := i.Libraries.FilePath(ctx, track)
	if err != nil {
		return err
	}
	posted, err := i.Sender.Post(ctx, chatID, path, track)
	if err != nil {
		return err
	}
	return i.Tx.WithinTx(ctx, func(ctx context.Context) error {
		return libraries.RecordPosted(ctx, i.Tracks, track, posted)
	})
}
