package search_music

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

// SearchOwnTracks lists the newest first for an empty text.
type SearchOwnTracks struct {
	IDs       common.IDProvider
	Tracks    repositories.Tracks
	Libraries *libraries.Libraries
	Attached  *libraries.Attached
}

func (i *SearchOwnTracks) Execute(ctx context.Context, text string, offset, limit int) (*OwnPage[library.Track], error) {
	_, _, kept, err := libraries.CurrentKept(ctx, i.IDs, i.Libraries, i.Attached)
	if err != nil {
		return nil, err
	}
	tracks, err := i.Tracks.Search(ctx, libraries.IDs(kept), text, offset, limit+1)
	if err != nil {
		return nil, err
	}
	return page(tracks, limit), nil
}
