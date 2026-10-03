package manage_quotas

import (
	"context"

	"github.com/lubaskinc0de/beatstash/internal/application/common"
	"github.com/lubaskinc0de/beatstash/internal/application/common/repositories"
	"github.com/lubaskinc0de/beatstash/internal/domain/library"
)

// SetUserQuota gives the User an own Quota. A Quota below what the User
// keeps deletes nothing: it only refuses new Tracks.
type SetUserQuota struct {
	IDs       common.IDProvider
	Libraries repositories.Libraries
}

func (i *SetUserQuota) Execute(ctx context.Context, userID uint, q *library.Quota) error {
	if _, err := common.CurrentAdmin(ctx, i.IDs); err != nil {
		return err
	}
	personal, err := i.Libraries.Personal(ctx, userID)
	if err != nil {
		return err
	}
	if err := personal.SetQuota(q); err != nil {
		return err
	}
	return i.Libraries.Save(ctx, personal)
}
