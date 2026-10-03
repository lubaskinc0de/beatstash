package send_listen_link

import (
	"context"

	"github.com/lubaskinc0de/beatstash/internal/application/common"
	"github.com/lubaskinc0de/beatstash/internal/application/common/listening"
)

// CheckListenLinks returns why the user cannot send a Track without a
// Telegram file.
type CheckListenLinks struct {
	IDs         common.IDProvider
	ListenLinks *listening.ListenLinks
}

func (i *CheckListenLinks) Execute(ctx context.Context) error {
	user, err := i.IDs.CurrentUser(ctx)
	if err != nil {
		return err
	}
	return i.ListenLinks.Check(ctx, user.ID)
}
