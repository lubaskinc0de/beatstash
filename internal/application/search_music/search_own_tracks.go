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
}

func (i *SearchOwnTracks) Execute(ctx context.Context, text string, offset, limit int) (*OwnPage[library.Track], error) {
	user, err := i.IDs.CurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	libs, err := i.Libraries.Of(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	tracks, err := i.Tracks.Search(ctx, libs.KeptIDs(), text, offset, limit+1)
	if err != nil {
		return nil, err
	}
	return page(tracks, limit), nil
}
