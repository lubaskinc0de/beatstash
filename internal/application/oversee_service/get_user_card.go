package oversee_service

import (
	"context"

	"github.com/lubaskinc0de/beatstash/internal/application/common"
	"github.com/lubaskinc0de/beatstash/internal/application/common/quotas"
	"github.com/lubaskinc0de/beatstash/internal/application/common/repositories"
	"github.com/lubaskinc0de/beatstash/internal/domain/access"
	"github.com/lubaskinc0de/beatstash/internal/domain/library"
)

type UserCard struct {
	User *access.User
	// Tracks and Usage are of the Personal Library.
	Tracks int64
	Usage  library.Usage
	// OwnQuota is false when the User follows the Default Quota.
	OwnQuota bool
}

type UserGetter interface {
	// Get loads the User with their Identities.
	Get(ctx context.Context, id uint) (*access.User, error)
}

type GetUserCard struct {
	IDs       common.IDProvider
	Users     UserGetter
	Libraries repositories.Libraries
	Tracks    repositories.Tracks
	Quotas    *quotas.Quotas
}

func (i *GetUserCard) Execute(ctx context.Context, userID uint) (*UserCard, error) {
	if _, err := common.CurrentAdmin(ctx, i.IDs); err != nil {
		return nil, err
	}
	user, err := i.Users.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	personal, err := i.Libraries.Personal(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	tracks, err := i.Tracks.CountIn(ctx, []uint{personal.ID})
	if err != nil {
		return nil, err
	}
	usage, err := i.Quotas.UsageOf(ctx, personal)
	if err != nil {
		return nil, err
	}
	return &UserCard{User: user, Tracks: tracks, Usage: usage, OwnQuota: personal.Quota != nil}, nil
}
