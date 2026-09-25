package greet_stranger

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
)

type ServiceStats struct {
	Users        int64
	SharedTracks int64
}

type GetServiceStats struct {
	Users     UserCounter
	Tracks    TrackCounter
	Libraries repositories.Libraries
}

type UserCounter interface {
	Count(ctx context.Context) (int64, error)
}

type TrackCounter interface {
	Count(ctx context.Context, libraryID uint) (int64, error)
}

func (i *GetServiceStats) Execute(ctx context.Context) (ServiceStats, error) {
	users, err := i.Users.Count(ctx)
	if err != nil {
		return ServiceStats{}, err
	}
	shared, err := i.Libraries.Shared(ctx)
	if err != nil {
		return ServiceStats{}, err
	}
	tracks, err := i.Tracks.Count(ctx, shared.ID)
	if err != nil {
		return ServiceStats{}, err
	}
	return ServiceStats{Users: users, SharedTracks: tracks}, nil
}
