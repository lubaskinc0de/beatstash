package library

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

var (
	ErrNotOwnTrack      = errors.New("track is not in the user's Personal Library")
	ErrInboxTrack       = errors.New("inbox track cannot be shared")
	ErrNotShared        = errors.New("track is not in the Shared Library")
	ErrAlreadyInLibrary = errors.New("the track is in the user's Personal Library already")
)

type Sharing struct {
	Tx        application.TxManager
	Tracks    application.TrackRepository
	Shares    application.ShareRepository
	Takes     application.TakeRepository
	Libraries *application.Libraries
	Lock      application.LibraryLock
	Clock     func() time.Time
}

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
}

func (s *Sharing) Find(ctx context.Context, ref domain.TrackRef) (*ShareState, error) {
	sc, err := s.scope(ctx)
	if err != nil {
		return nil, err
	}
	source, err := s.Tracks.FindSource(ctx, sc.libs.Personal.ID, ref.Provider, ref.ID)
	if err != nil {
		return nil, err
	}
	return s.State(ctx, source.TrackID)
}

func (s *Sharing) State(ctx context.Context, trackID uint) (*ShareState, error) {
	sc, err := s.scope(ctx)
	if err != nil {
		return nil, err
	}
	track, err := s.own(ctx, sc, trackID)
	if err != nil {
		return nil, err
	}
	if !track.Complete() {
		return nil, ErrInboxTrack
	}

	shared, err := s.isShared(ctx, track.ID)
	if err != nil {
		return nil, err
	}
	state := &ShareState{TrackID: track.ID, Shared: shared, HasAlbum: track.Album != ""}
	if !state.HasAlbum {
		return state, nil
	}

	album, err := s.album(ctx, sc, track)
	if err != nil {
		return nil, err
	}
	state.AlbumShared = true
	for _, t := range album {
		shared, err := s.isShared(ctx, t.ID)
		if err != nil {
			return nil, err
		}
		state.AlbumShared = state.AlbumShared && shared
	}
	return state, nil
}

// Share shares the user's Track, or with wholeAlbum every complete Track
// of its album.
func (s *Sharing) Share(ctx context.Context, trackID uint, wholeAlbum bool) (ShareResult, error) {
	var result ShareResult
	err := s.eachTrack(ctx, trackID, wholeAlbum, func(ctx context.Context, sc *scope, track *domain.Track) error {
		return s.share(ctx, sc, track, &result)
	})
	return result, err
}

func (s *Sharing) Unshare(ctx context.Context, trackID uint, wholeAlbum bool) error {
	return s.eachTrack(ctx, trackID, wholeAlbum, s.unshare)
}

func (s *Sharing) eachTrack(
	ctx context.Context,
	trackID uint,
	wholeAlbum bool,
	fn func(context.Context, *scope, *domain.Track) error,
) error {
	return s.within(ctx, func(ctx context.Context, sc *scope) error {
		track, err := s.own(ctx, sc, trackID)
		if err != nil {
			return err
		}
		if !wholeAlbum {
			return fn(ctx, sc, track)
		}
		album, err := s.album(ctx, sc, track)
		if err != nil {
			return err
		}
		for i := range album {
			if err := fn(ctx, sc, &album[i]); err != nil {
				return err
			}
		}
		return nil
	})
}

// share copies the Track into the Shared Library. If a Duplicate is there
// already, no second copy appears: the user's Share points at the existing
// Track, stays out of the Top and keeps the Track shared once its author
// unshares.
func (s *Sharing) share(ctx context.Context, sc *scope, track *domain.Track, result *ShareResult) error {
	if !track.Complete() {
		return ErrInboxTrack
	}
	if shared, err := s.isShared(ctx, track.ID); err != nil || shared {
		return err
	}

	share := &domain.Share{SourceTrackID: track.ID, UserID: sc.user.ID, CreatedAt: s.Clock()}
	duplicate, err := s.Tracks.FindDuplicate(ctx, sc.libs.Shared.ID, track.Metadata, track.DurationMs)
	switch {
	case err == nil:
		sharers, err := s.Shares.ForTrack(ctx, duplicate.ID)
		if err != nil {
			return err
		}
		if result.AlreadyShared == 0 && len(sharers) > 0 {
			result.Author = &sharers[0].User
		}
		result.AlreadyShared++
		share.TrackID = duplicate.ID
		return s.Shares.Save(ctx, share)
	case !errors.Is(err, application.ErrTrackNotFound):
		return err
	}

	copied, target, err := s.copyTrack(ctx, track, sc.libs.Personal, sc.libs.Shared)
	if err != nil {
		return err
	}
	sc.onRollback = append(sc.onRollback, func() { RemoveFile(target) })
	result.Created++
	share.TrackID = copied.ID
	share.InTop = true
	return s.Shares.Save(ctx, share)
}

// unshare drops the user's Share; the Track leaves the Shared Library only
// with its last sharer.
func (s *Sharing) unshare(ctx context.Context, sc *scope, track *domain.Track) error {
	share, err := s.Shares.BySource(ctx, track.ID)
	if errors.Is(err, application.ErrShareNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := s.Shares.Delete(ctx, share.ID); err != nil {
		return err
	}

	rest, err := s.Shares.ForTrack(ctx, share.TrackID)
	if err != nil || len(rest) > 0 {
		return err
	}
	shared, err := s.Tracks.Get(ctx, share.TrackID)
	if err != nil {
		return err
	}
	if err := s.Tracks.Delete(ctx, shared.ID); err != nil {
		return err
	}
	file := filepath.Join(s.Libraries.Dir(sc.libs.Shared), shared.Path)
	sc.afterCommit = append(sc.afterCommit, func() { RemoveFile(file) })
	return nil
}

func (s *Sharing) isShared(ctx context.Context, trackID uint) (bool, error) {
	_, err := s.Shares.BySource(ctx, trackID)
	if errors.Is(err, application.ErrShareNotFound) {
		return false, nil
	}
	return err == nil, err
}

// album lists the complete Tracks of the track's album; a single makes an
// album of one.
func (s *Sharing) album(ctx context.Context, sc *scope, track *domain.Track) ([]domain.Track, error) {
	if track.Album == "" {
		return []domain.Track{*track}, nil
	}
	tracks, err := s.Tracks.Album(ctx, sc.libs.Personal.ID, track.AlbumArtist, track.Album)
	if err != nil {
		return nil, err
	}
	complete := tracks[:0]
	for _, t := range tracks {
		if t.Complete() {
			complete = append(complete, t)
		}
	}
	return complete, nil
}

func (s *Sharing) own(ctx context.Context, sc *scope, trackID uint) (*domain.Track, error) {
	track, err := s.Tracks.Get(ctx, trackID)
	if errors.Is(err, application.ErrTrackNotFound) {
		return nil, ErrNotOwnTrack
	}
	if err != nil {
		return nil, err
	}
	if track.LibraryID != sc.libs.Personal.ID {
		return nil, ErrNotOwnTrack
	}
	return track, nil
}

type scope struct {
	user *domain.User
	libs application.UserLibraries

	onRollback  []func()
	afterCommit []func()
}

func (s *Sharing) scope(ctx context.Context) (*scope, error) {
	user, ok := application.UserFromContext(ctx)
	if !ok {
		return nil, application.ErrNotAuthenticated
	}
	libs, err := s.Libraries.Of(ctx, user)
	if err != nil {
		return nil, err
	}
	return &scope{user: user, libs: libs}, nil
}

// within runs fn in one transaction holding both libraries; file changes
// are undone if it fails and finished once it commits.
func (s *Sharing) within(ctx context.Context, fn func(context.Context, *scope) error) error {
	sc, err := s.scope(ctx)
	if err != nil {
		return err
	}
	err = s.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.Lock.Lock(ctx, sc.libs.IDs()...); err != nil {
			return err
		}
		return fn(ctx, sc)
	})
	if err != nil {
		for _, undo := range sc.onRollback {
			undo()
		}
		return err
	}
	for _, finish := range sc.afterCommit {
		finish()
	}
	return nil
}
