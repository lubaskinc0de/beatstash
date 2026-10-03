package search_music

import (
	"context"

	"github.com/lubaskinc0de/beatstash/internal/application/common"
	"github.com/lubaskinc0de/beatstash/internal/application/common/libraries"
	"github.com/lubaskinc0de/beatstash/internal/application/common/repositories"
)

type SearchOwnAlbums struct {
	IDs       common.IDProvider
	Tracks    repositories.Tracks
	Libraries *libraries.Libraries
}

func (i *SearchOwnAlbums) Execute(ctx context.Context, text string, offset, limit int) (*OwnPage[repositories.AlbumSummary], error) {
	user, err := i.IDs.CurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	libs, err := i.Libraries.Of(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	albums, err := i.Tracks.SearchAlbums(ctx, libs.KeptIDs(), text, false, offset, limit+1)
	if err != nil {
		return nil, err
	}
	return page(albums, limit), nil
}
