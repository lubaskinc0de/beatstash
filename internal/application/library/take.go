package library

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/lubaskinc0de/navidrome-tg/internal/application"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

func (s *Sharing) Take(ctx context.Context, sharedTrackID uint) error {
	return s.within(ctx, func(ctx context.Context, sc *scope) error {
		track, err := s.sharedTrack(ctx, sc.libs.Shared, sharedTrackID)
		if err != nil {
			return err
		}

		_, err = s.Tracks.FindDuplicate(ctx, sc.libs.Personal.ID, track.Metadata, track.DurationMs)
		if err == nil {
			return ErrAlreadyInLibrary
		}
		if !errors.Is(err, application.ErrTrackNotFound) {
			return err
		}

		sharers, err := s.Shares.ForTrack(ctx, track.ID)
		if err != nil {
			return err
		}
		copied, _, err := s.CopyShared(ctx, track, sc.libs, &sc.files)
		if err != nil {
			return err
		}

		take := &domain.Take{UserID: sc.user.ID, TrackID: copied.ID, CreatedAt: s.Clock()}
		if len(sharers) > 0 {
			take.AuthorID = &sharers[0].UserID
		}
		return s.Takes.Save(ctx, take)
	})
}

// CopyShared puts a Shared Library Track into the user's Personal Library
// without a Take: Ingest uses it when the user sends the very file somebody
// shared. The caller holds a transaction and the locks of both libraries.
func (s *Sharing) CopyShared(
	ctx context.Context,
	track *domain.Track,
	libs application.UserLibraries,
	files *FileChanges,
) (copied *domain.Track, target string, err error) {
	return s.copyTrack(ctx, track, libs.Shared, libs.Personal, files)
}

// copyTrack hardlinks the Track's file into another library and gives the
// copy the same Track Refs, so a known ref still needs no download.
func (s *Sharing) copyTrack(
	ctx context.Context,
	track *domain.Track,
	from, to *domain.Library,
	files *FileChanges,
) (*domain.Track, string, error) {
	dir := s.Libraries.Dir(to)
	rel, err := FreePath(dir, track.Path)
	if err != nil {
		return nil, "", err
	}
	target := filepath.Join(dir, rel)
	if err := Link(filepath.Join(s.Libraries.Dir(from), track.Path), target); err != nil {
		return nil, "", err
	}
	files.OnRollback(func() { RemoveFile(target) })

	copied := &domain.Track{
		LibraryID:  to.ID,
		Path:       rel,
		Metadata:   track.Metadata,
		Quality:    track.Quality,
		DurationMs: track.DurationMs,
		Format:     track.Format,
	}
	if err := s.Tracks.SaveTrack(ctx, copied); err != nil {
		return nil, "", err
	}

	sources, err := s.Tracks.Sources(ctx, track.ID)
	if err != nil {
		return nil, "", err
	}
	for _, source := range sources {
		_, err := s.Tracks.FindSource(ctx, to.ID, source.Provider, source.Ref)
		if err == nil {
			continue
		}
		if !errors.Is(err, application.ErrSourceNotFound) {
			return nil, "", err
		}
		source.ID = 0
		source.TrackID = copied.ID
		source.LibraryID = to.ID
		if err := s.Tracks.SaveSource(ctx, &source); err != nil {
			return nil, "", err
		}
	}
	return copied, target, nil
}

func (s *Sharing) sharedTrack(ctx context.Context, shared *domain.Library, trackID uint) (*domain.Track, error) {
	track, err := s.Tracks.Get(ctx, trackID)
	if errors.Is(err, application.ErrTrackNotFound) {
		return nil, ErrNotShared
	}
	if err != nil {
		return nil, err
	}
	if track.LibraryID != shared.ID {
		return nil, ErrNotShared
	}
	return track, nil
}
