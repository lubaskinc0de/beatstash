package libraries

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

// CopyTrack expects a transaction holding both libraries. The copy gets the
// Track's Sources the target Library lacks.
func CopyTrack(
	ctx context.Context,
	tracks repositories.Tracks,
	disk common.Disk,
	musicDir string,
	track *library.Track,
	from, to *library.Library,
	changes *FileChanges,
) (copied *library.Track, target string, err error) {
	dir := Dir(musicDir, to)
	rel, err := disk.FreePath(dir, track.Path)
	if err != nil {
		return nil, "", err
	}
	target = filepath.Join(dir, rel)
	if err := changes.Link(filepath.Join(Dir(musicDir, from), track.Path), target); err != nil {
		return nil, "", err
	}

	copied = track.CopyTo(to, rel)
	for _, source := range track.Sources {
		_, err := tracks.FindSource(ctx, to.ID, source.Provider, source.Ref)
		if err == nil {
			continue
		}
		if !errors.Is(err, repositories.ErrSourceNotFound) {
			return nil, "", err
		}
		copied.AddSource(source.TrackRef())
	}
	if err := tracks.SaveTrack(ctx, copied); err != nil {
		return nil, "", err
	}
	return copied, target, nil
}
