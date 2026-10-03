package share_tracks

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/sharing"
)

type TrackCard struct {
	Track   *library.Track
	Library *library.Library
	Shared  bool
	// Author shared the Track or its Duplicate first; nil if nobody did, or
	// if it is the user.
	Author *access.User
}

type ViewTrackCard struct {
	IDs       common.IDProvider
	Tracks    repositories.Tracks
	Shared    repositories.SharedTracks
	Libraries *libraries.Libraries
}

func (i *ViewTrackCard) Execute(ctx context.Context, trackID uint) (*TrackCard, error) {
	user, err := i.IDs.CurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	libs, err := i.Libraries.Of(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	track, err := keptTrack(ctx, i.Tracks, libs.Kept(), trackID)
	if err != nil {
		return nil, err
	}
	card := &TrackCard{Track: track, Library: libraryOf(track, libs.Kept())}

	shared, err := i.Shared.BySource(ctx, track.ID)
	switch {
	case err == nil:
		card.Shared = true
	case !errors.Is(err, sharing.ErrNotShared):
		return nil, err
	default:
		shared, err = i.sharedDuplicate(ctx, libs.Shared, track)
		if err != nil || shared == nil {
			return card, err
		}
	}
	if author := shared.Author(); author != nil && author.UserID != user.ID {
		card.Author = &author.User
	}
	return card, nil
}

// sharedDuplicate returns nil without a Duplicate in the Shared Library.
func (i *ViewTrackCard) sharedDuplicate(ctx context.Context, sharedLib *library.Library, track *library.Track) (*sharing.SharedTrack, error) {
	duplicate, err := i.Tracks.FindDuplicate(ctx, []uint{sharedLib.ID}, track.Metadata, track.DurationMs)
	if errors.Is(err, repositories.ErrTrackNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	shared, err := i.Shared.Get(ctx, duplicate.ID)
	if errors.Is(err, sharing.ErrNotShared) {
		return nil, nil
	}
	return shared, err
}
