package search_music

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
)

type SearchOwnAlbums struct {
	IDs       common.IDProvider
	Tracks    repositories.Tracks
	Libraries *libraries.Libraries
	Attached  *libraries.Attached
}

func (i *SearchOwnAlbums) Execute(ctx context.Context, text string, offset, limit int) (*OwnPage[repositories.AlbumSummary], error) {
	_, _, kept, err := libraries.CurrentKept(ctx, i.IDs, i.Libraries, i.Attached)
	if err != nil {
		return nil, err
	}
	albums, err := i.Tracks.SearchAlbums(ctx, libraries.IDs(kept), text, false, offset, limit+1)
	if err != nil {
		return nil, err
	}
	return page(albums, limit), nil
}
