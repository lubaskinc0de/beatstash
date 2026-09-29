package share_tracks

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/navidrome"
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

// ShareDeps is what ShareTrack and ShareAlbum need.
type ShareDeps struct {
	IDs       common.IDProvider
	Tx        repositories.TxManager
	Lock      repositories.LibraryLock
	Tracks    repositories.Tracks
	Shared    repositories.SharedTracks
	Libraries *libraries.Libraries
	Attached  *libraries.Attached
	Navidrome navidrome.Client
	// Admin downloads the files of Attached Libraries.
	Admin    navidrome.Credentials
	Disk     common.Disk
	MusicDir string
	Clock    func() time.Time
}

// within runs fn in a transaction holding the current user's Managed
// Libraries, with a sharer for their Kept Libraries.
func (d *ShareDeps) within(ctx context.Context, fn func(ctx context.Context, s *sharer) error) error {
	user, libs, kept, err := libraries.CurrentKept(ctx, d.IDs, d.Libraries, d.Attached)
	if err != nil {
		return err
	}
	return libraries.Within(ctx, d.Tx, d.Lock, d.Disk, libs, func(ctx context.Context, changes *libraries.FileChanges) error {
		return fn(ctx, &sharer{
			tracks:    d.Tracks,
			shared:    d.Shared,
			lock:      d.Lock,
			disk:      d.Disk,
			navidrome: d.Navidrome,
			admin:     d.Admin,
			musicDir:  d.MusicDir,
			user:      user,
			libs:      libs,
			kept:      kept,
			changes:   changes,
			now:       d.Clock(),
		})
	})
}

// sharer shares Tracks of one user inside one libraries.Within transaction.
// Call lockAttached before share.
type sharer struct {
	tracks    repositories.Tracks
	shared    repositories.SharedTracks
	lock      repositories.LibraryLock
	disk      common.Disk
	navidrome navidrome.Client
	admin     navidrome.Credentials
	musicDir  string
	user      *access.User
	libs      libraries.ManagedLibraries
	kept      []*library.Library
	changes   *libraries.FileChanges
	now       time.Time
}

// lockAttached keeps a refresh of the Attached Library from deleting the
// tracks while they are copied, and drops the ones gone already. Tracks of
// an album share one library.
func (s *sharer) lockAttached(ctx context.Context, tracks []library.Track) ([]library.Track, error) {
	if len(tracks) == 0 || !tracks[0].Attached() {
		return tracks, nil
	}
	if err := s.lock.Lock(ctx, tracks[0].LibraryID); err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(tracks))
	for _, t := range tracks {
		ids = append(ids, t.ID)
	}
	return s.tracks.GetMany(ctx, ids)
}

func (s *sharer) share(ctx context.Context, track *library.Track, result *ShareResult) error {
	if err := track.ShareableBy(s.kept); err != nil {
		return err
	}
	if shared, err := isShared(ctx, s.shared, track.ID); err != nil || shared {
		return err
	}

	duplicate, err := s.tracks.FindDuplicate(ctx, []uint{s.libs.Shared.ID}, track.Metadata, track.DurationMs)
	switch {
	case err == nil:
		shared, err := s.shared.Get(ctx, duplicate.ID)
		if err != nil {
			return err
		}
		if author := shared.Author(); result.AlreadyShared == 0 && author != nil {
			result.Author = &author.User
		}
		result.AlreadyShared++
		shared.ShareBy(s.user, track, s.now)
		return s.shared.Save(ctx, shared)
	case !errors.Is(err, repositories.ErrTrackNotFound):
		return err
	}

	var copied *library.Track
	if track.Attached() {
		copied, err = s.copyAttached(ctx, track)
	} else {
		copied, _, err = libraries.CopyTrack(ctx, s.tracks, s.disk, s.musicDir, track, s.libs.Personal, s.libs.Shared, s.changes)
	}
	if err != nil {
		return err
	}
	result.Created++
	return s.shared.Save(ctx, sharing.NewSharedTrack(copied, s.user, track, s.now))
}

// copyAttached copies the file of an Attached Track, which lies out of the
// bot's reach, through Navidrome; the copy goes where Layout puts it.
func (s *sharer) copyAttached(ctx context.Context, track *library.Track) (*library.Track, error) {
	body, err := s.navidrome.Download(ctx, s.admin, track.SongID)
	if err != nil {
		return nil, err
	}
	staged, err := s.disk.Stage(body, track.Format.Ext())
	_ = body.Close()
	if err != nil {
		return nil, err
	}
	defer s.disk.Remove(staged)

	dir := libraries.Dir(s.musicDir, s.libs.Shared)
	rel, err := s.disk.FreePath(dir, library.LayoutPath(track.Metadata, track.Format, filepath.Base(track.Path)))
	if err != nil {
		return nil, err
	}
	if err := s.changes.Place(staged, filepath.Join(dir, rel)); err != nil {
		return nil, err
	}
	return libraries.SaveCopy(ctx, s.tracks, track, s.libs.Shared, rel)
}

// unshare takes the user's Share of the Track back. Once nobody shares it,
// the file leaves sharedDir after the commit.
func unshare(
	ctx context.Context,
	shared repositories.SharedTracks,
	sharedDir string,
	changes *libraries.FileChanges,
	track *library.Track,
) error {
	sharedTrack, err := shared.BySource(ctx, track.ID)
	if errors.Is(err, sharing.ErrNotShared) {
		return nil
	}
	if err != nil {
		return err
	}
	if !sharedTrack.Unshare(track) {
		return shared.Save(ctx, sharedTrack)
	}
	if err := shared.Delete(ctx, sharedTrack); err != nil {
		return err
	}
	changes.RemoveAfterCommit(filepath.Join(sharedDir, sharedTrack.Track.Path))
	return nil
}

// keptTrack returns ErrNotKeptTrack unless libs hold the Track.
func keptTrack(ctx context.Context, tracks repositories.Tracks, libs []*library.Library, trackID uint) (*library.Track, error) {
	track, err := tracks.Get(ctx, trackID)
	if errors.Is(err, repositories.ErrTrackNotFound) {
		return nil, library.ErrNotKeptTrack
	}
	if err != nil {
		return nil, err
	}
	if err := track.KeptIn(libs); err != nil {
		return nil, err
	}
	return track, nil
}

// album lists the shareable Tracks of the track's album in its library; a
// single makes an album of one.
func album(ctx context.Context, tracks repositories.Tracks, kept []*library.Library, track *library.Track) ([]library.Track, error) {
	if track.Single() {
		return []library.Track{*track}, nil
	}
	all, err := tracks.Album(ctx, track.LibraryID, track.AlbumArtist, track.Album)
	if err != nil {
		return nil, err
	}
	var shareable []library.Track
	for _, t := range all {
		if t.ShareableBy(kept) == nil {
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

// shareState tells what is shared of the track and of its album, as album
// lists it.
func shareState(
	ctx context.Context,
	sharedTracks repositories.SharedTracks,
	track *library.Track,
	albumTracks []library.Track,
) (*ShareState, error) {
	ids := []uint{track.ID}
	for _, t := range albumTracks {
		ids = append(ids, t.ID)
	}
	shared, err := sharedTracks.SharedSources(ctx, ids)
	if err != nil {
		return nil, err
	}
	state := &ShareState{TrackID: track.ID, Shared: shared[track.ID], HasAlbum: !track.Single()}
	state.AlbumShared = state.HasAlbum
	for _, t := range albumTracks {
		state.AlbumShared = state.AlbumShared && shared[t.ID]
	}
	return state, nil
}
