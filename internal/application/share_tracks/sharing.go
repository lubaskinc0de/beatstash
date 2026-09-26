package share_tracks

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/sharing"
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
	Author        *access.User
	State         *ShareState
}

// operation runs under libraries.Within.
type operation struct {
	tracks   repositories.Tracks
	shared   repositories.SharedTracks
	disk     common.Disk
	musicDir string
	user     *access.User
	libs     libraries.UserLibraries
	changes  *libraries.FileChanges
	now      time.Time
}

func (op *operation) share(ctx context.Context, track *library.Track, result *ShareResult) error {
	if err := track.ShareableBy(op.libs.Personal); err != nil {
		return err
	}
	if shared, err := isShared(ctx, op.shared, track.ID); err != nil || shared {
		return err
	}

	duplicate, err := op.tracks.FindDuplicate(ctx, op.libs.Shared.ID, track.Metadata, track.DurationMs)
	switch {
	case err == nil:
		shared, err := op.shared.Get(ctx, duplicate.ID)
		if err != nil {
			return err
		}
		if author := shared.Author(); result.AlreadyShared == 0 && author != nil {
			result.Author = &author.User
		}
		result.AlreadyShared++
		shared.ShareBy(op.user, track, op.now)
		return op.shared.Save(ctx, shared)
	case !errors.Is(err, repositories.ErrTrackNotFound):
		return err
	}

	copied, _, err := libraries.CopyTrack(ctx, op.tracks, op.disk, op.musicDir, track, op.libs.Personal, op.libs.Shared, op.changes)
	if err != nil {
		return err
	}
	result.Created++
	return op.shared.Save(ctx, sharing.NewSharedTrack(copied, op.user, track, op.now))
}

func (op *operation) unshare(ctx context.Context, track *library.Track) error {
	shared, err := op.shared.BySource(ctx, track.ID)
	if errors.Is(err, sharing.ErrNotShared) {
		return nil
	}
	if err != nil {
		return err
	}
	if !shared.Unshare(track) {
		return op.shared.Save(ctx, shared)
	}
	if err := op.shared.Delete(ctx, shared); err != nil {
		return err
	}
	file := filepath.Join(libraries.Dir(op.musicDir, op.libs.Shared), shared.Track.Path)
	op.changes.RemoveAfterCommit(file)
	return nil
}

func own(ctx context.Context, tracks repositories.Tracks, personal *library.Library, trackID uint) (*library.Track, error) {
	track, err := tracks.Get(ctx, trackID)
	if errors.Is(err, repositories.ErrTrackNotFound) {
		return nil, library.ErrNotOwnTrack
	}
	if err != nil {
		return nil, err
	}
	if err := track.OwnedBy(personal); err != nil {
		return nil, err
	}
	return track, nil
}

// album lists the shareable Tracks of the track's album; a single makes an
// album of one.
func album(ctx context.Context, tracks repositories.Tracks, personal *library.Library, track *library.Track) ([]library.Track, error) {
	if track.Single() {
		return []library.Track{*track}, nil
	}
	all, err := tracks.Album(ctx, personal.ID, track.AlbumArtist, track.Album)
	if err != nil {
		return nil, err
	}
	var shareable []library.Track
	for _, t := range all {
		if t.ShareableBy(personal) == nil {
			shareable = append(shareable, t)
		}
	}
	return shareable, nil
}

func isShared(ctx context.Context, shared repositories.SharedTracks, trackID uint) (bool, error) {
	_, err := shared.BySource(ctx, trackID)
	if errors.Is(err, sharing.ErrNotShared) {
		return false, nil
	}
	return err == nil, err
}

func shareState(
	ctx context.Context,
	tracks repositories.Tracks,
	sharedTracks repositories.SharedTracks,
	personal *library.Library,
	track *library.Track,
) (*ShareState, error) {
	shared, err := isShared(ctx, sharedTracks, track.ID)
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
		shared, err := isShared(ctx, sharedTracks, t.ID)
		if err != nil {
			return nil, err
		}
		state.AlbumShared = state.AlbumShared && shared
	}
	return state, nil
}
