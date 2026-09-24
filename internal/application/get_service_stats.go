package application

import "context"

type ServiceStats struct {
	Users        int64
	SharedTracks int64
}

type UserCounter interface {
	Count(ctx context.Context) (int64, error)
}

type TrackCounter interface {
	Count(ctx context.Context, libraryID uint) (int64, error)
}

type GetServiceStats struct {
	Users     UserCounter
	Tracks    TrackCounter
	Libraries LibraryRepository
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
