package libraries

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

// CopyTrack expects a transaction holding both libraries.
func CopyTrack(
	ctx context.Context,
	tracks repositories.Tracks,
	disk common.Disk,
	musicDir string,
	track *domain.Track,
	from, to *domain.Library,
	changes *FileChanges,
) (copied *domain.Track, target string, err error) {
	dir := Dir(musicDir, to)
	rel, err := disk.FreePath(dir, track.Path)
	if err != nil {
		return nil, "", err
	}
	target = filepath.Join(dir, rel)
	if err := disk.Link(filepath.Join(Dir(musicDir, from), track.Path), target); err != nil {
		return nil, "", err
	}
	changes.OnRollback(func() { disk.Remove(target) })

	copied = track.CopyTo(to, rel)
	if err := tracks.SaveTrack(ctx, copied); err != nil {
		return nil, "", err
	}

	sources, err := tracks.Sources(ctx, track.ID)
	if err != nil {
		return nil, "", err
	}
	for _, source := range sources {
		if err := saveSource(ctx, tracks, source.For(copied)); err != nil {
			return nil, "", err
		}
	}
	return copied, target, nil
}

// RecordPosted gives the posted file_id to the Track and its copies, so
// nothing is posted twice.
func RecordPosted(ctx context.Context, tracks repositories.Tracks, track *domain.Track, posted *common.PostedFile) error {
	copies, err := tracks.Copies(ctx, track.ID)
	if err != nil {
		return err
	}
	for i := range copies {
		if err := saveSource(ctx, tracks, domain.TelegramSource(&copies[i], posted.UniqueID, posted.File)); err != nil {
			return err
		}
	}
	return nil
}

func saveSource(ctx context.Context, tracks repositories.Tracks, source *domain.TrackSource) error {
	_, err := tracks.FindSource(ctx, source.LibraryID, source.Provider, source.Ref)
	if err == nil {
		return nil
	}
	if !errors.Is(err, repositories.ErrSourceNotFound) {
		return err
	}
	return tracks.SaveSource(ctx, source)
}
