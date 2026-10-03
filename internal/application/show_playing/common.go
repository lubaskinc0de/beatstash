package show_playing

import (
	"context"

	"github.com/lubaskinc0de/beatstash/internal/application/common/navidrome"
	"github.com/lubaskinc0de/beatstash/internal/application/common/repositories"
	"github.com/lubaskinc0de/beatstash/internal/domain/library"
)

// findTracks finds the Track of each played song, nil where none: by
// metadata among the libraries, then by song id among the Attached ones.
func findTracks(
	ctx context.Context,
	tracks repositories.Tracks,
	libraryIDs, attached []uint,
	played []navidrome.Track,
) ([]*library.Track, error) {
	ms := make([]library.Metadata, 0, len(played))
	for _, song := range played {
		ms = append(ms, song.Metadata())
	}
	found, err := tracks.FindByMetadata(ctx, libraryIDs, ms)
	if err != nil {
		return nil, err
	}
	var missing []string
	for n, song := range played {
		if found[n] == nil {
			missing = append(missing, song.ID)
		}
	}
	if len(missing) == 0 || len(attached) == 0 {
		return found, nil
	}
	bySong, err := tracks.BySongs(ctx, attached, missing)
	if err != nil {
		return nil, err
	}
	for n, song := range played {
		if found[n] == nil {
			found[n] = bySong[song.ID]
		}
	}
	return found, nil
}
