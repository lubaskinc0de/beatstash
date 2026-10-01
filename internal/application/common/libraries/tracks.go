package libraries

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

// ReplaceTracks deletes the gone Tracks before it saves the others: a saved
// Track may take the path of a gone one.
func ReplaceTracks(ctx context.Context, tracks repositories.Tracks, save, gone []*library.Track) error {
	ids := make([]uint, 0, len(gone))
	for _, track := range gone {
		ids = append(ids, track.ID)
	}
	if err := tracks.Delete(ctx, ids); err != nil {
		return err
	}
	return tracks.SaveTracks(ctx, save)
}
