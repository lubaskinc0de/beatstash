package libraries

import (
	"context"
	"path/filepath"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
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

	copied, err = SaveCopy(ctx, tracks, track, to, rel)
	return copied, target, err
}

// SaveCopy saves the copy with the Track's Sources the target Library lacks.
func SaveCopy(ctx context.Context, tracks repositories.Tracks, track *library.Track, to *library.Library, rel string) (*library.Track, error) {
	copied := track.CopyTo(to, rel)
	refs := make([]provider.TrackRef, 0, len(track.Sources))
	for _, source := range track.Sources {
		refs = append(refs, source.TrackRef())
	}
	known, err := tracks.KnownSources(ctx, []uint{to.ID}, refs)
	if err != nil {
		return nil, err
	}
	for _, ref := range refs {
		if !known[ref] {
			copied.AddSource(ref)
		}
	}
	if err := tracks.SaveTrack(ctx, copied); err != nil {
		return nil, err
	}
	return copied, nil
}
