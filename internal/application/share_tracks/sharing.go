package share_tracks

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type ShareState struct {
	TrackID     uint
	Shared      bool
	HasAlbum    bool
	AlbumShared bool
}

type ShareResult struct {
	// Created counts Tracks that appeared in the Shared Library.
	Created int
	// AlreadyShared counts Tracks somebody else had shared; Author is the
	// one who shared the first of them.
	AlreadyShared int
	Author        *domain.User
	State         *ShareState
}

// operation runs under libraries.Within.
type operation struct {
	tracks   repositories.Tracks
	shares   repositories.Shares
	disk     common.Disk
	musicDir string
	user     *domain.User
	libs     libraries.UserLibraries
	changes  *libraries.FileChanges
	now      time.Time
}

func (op *operation) share(ctx context.Context, track *domain.Track, result *ShareResult) error {
	if err := track.Shareable(); err != nil {
		return err
	}
	if shared, err := isShared(ctx, op.shares, track.ID); err != nil || shared {
		return err
	}

	duplicate, err := op.tracks.FindDuplicate(ctx, op.libs.Shared.ID, track.Metadata, track.DurationMs)
	switch {
	case err == nil:
		sharers, err := op.shares.ForTrack(ctx, duplicate.ID)
		if err != nil {
			return err
		}
		if author := domain.Author(sharers); result.AlreadyShared == 0 && author != nil {
			result.Author = &author.User
		}
		result.AlreadyShared++
		return op.shares.Save(ctx, domain.ShareDuplicate(op.user.ID, track, duplicate, op.now))
	case !errors.Is(err, repositories.ErrTrackNotFound):
		return err
	}

	copied, _, err := libraries.CopyTrack(ctx, op.tracks, op.disk, op.musicDir, track, op.libs.Personal, op.libs.Shared, op.changes)
	if err != nil {
		return err
	}
	result.Created++
	return op.shares.Save(ctx, domain.ShareCopy(op.user.ID, track, copied, op.now))
}

// unshare drops the user's Share; the Track leaves the Shared Library only
// with its last sharer.
func (op *operation) unshare(ctx context.Context, track *domain.Track) error {
	share, err := op.shares.BySource(ctx, track.ID)
	if errors.Is(err, repositories.ErrShareNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := op.shares.Delete(ctx, share.ID); err != nil {
		return err
	}

	rest, err := op.shares.ForTrack(ctx, share.TrackID)
	if err != nil || len(rest) > 0 {
		return err
	}
	shared, err := op.tracks.Get(ctx, share.TrackID)
	if err != nil {
		return err
	}
	if err := op.tracks.Delete(ctx, shared.ID); err != nil {
		return err
	}
	file := filepath.Join(libraries.Dir(op.musicDir, op.libs.Shared), shared.Path)
	op.changes.AfterCommit(func() { op.disk.Remove(file) })
	return nil
}

func own(ctx context.Context, tracks repositories.Tracks, personal *domain.Library, trackID uint) (*domain.Track, error) {
	track, err := tracks.Get(ctx, trackID)
	if errors.Is(err, repositories.ErrTrackNotFound) {
		return nil, domain.ErrNotOwnTrack
	}
	if err != nil {
		return nil, err
	}
	if !track.In(personal) {
		return nil, domain.ErrNotOwnTrack
	}
	return track, nil
}

// album lists the shareable Tracks of the track's album; a single makes an
// album of one.
func album(ctx context.Context, tracks repositories.Tracks, personal *domain.Library, track *domain.Track) ([]domain.Track, error) {
	if track.Single() {
		return []domain.Track{*track}, nil
	}
	all, err := tracks.Album(ctx, personal.ID, track.AlbumArtist, track.Album)
	if err != nil {
		return nil, err
	}
	return domain.ShareableTracks(all), nil
}

func isShared(ctx context.Context, shares repositories.Shares, trackID uint) (bool, error) {
	_, err := shares.BySource(ctx, trackID)
	if errors.Is(err, repositories.ErrShareNotFound) {
		return false, nil
	}
	return err == nil, err
}

func shareState(
	ctx context.Context,
	tracks repositories.Tracks,
	shares repositories.Shares,
	personal *domain.Library,
	track *domain.Track,
) (*ShareState, error) {
	shared, err := isShared(ctx, shares, track.ID)
	if err != nil {
		return nil, err
	}
	state := &ShareState{TrackID: track.ID, Shared: shared, HasAlbum: !track.Single()}
	if !state.HasAlbum {
		return state, nil
	}

	album, err := album(ctx, tracks, personal, track)
	if err != nil {
		return nil, err
	}
	state.AlbumShared = true
	for _, t := range album {
		shared, err := isShared(ctx, shares, t.ID)
		if err != nil {
			return nil, err
		}
		state.AlbumShared = state.AlbumShared && shared
	}
	return state, nil
}
