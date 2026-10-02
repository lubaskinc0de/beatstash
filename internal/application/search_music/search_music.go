package search_music

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/listening"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

type Found struct {
	// Albums come on the first page only, and only with Listen Links: an
	// Album can be sent no other way.
	Albums   []repositories.AlbumSummary
	Tracks   []library.Track
	More     bool
	Linkable bool
}

// SearchMusic prefers a copy the user keeps to a shared one.
type SearchMusic struct {
	IDs         common.IDProvider
	Tracks      repositories.Tracks
	Libraries   *libraries.Libraries
	Attached    *libraries.Attached
	ListenLinks *listening.ListenLinks
}

const albumsShown = 3

func (i *SearchMusic) Execute(ctx context.Context, text string, offset, limit int) (*Found, error) {
	user, libs, kept, err := libraries.CurrentKept(ctx, i.IDs, i.Libraries, i.Attached)
	if err != nil {
		return nil, err
	}
	searched := append(libraries.IDs(kept), libs.Shared.ID)
	tracks, err := i.Tracks.Search(ctx, searched, text, offset, limit+1)
	if err != nil {
		return nil, err
	}
	found := &Found{}
	found.Tracks, found.More = cut(tracks, limit)
	found.Linkable, err = i.ListenLinks.Available(ctx, user.ID)
	if err != nil || !found.Linkable || offset > 0 {
		return found, err
	}
	found.Albums, err = i.Tracks.SearchAlbums(ctx, searched, text, true, 0, albumsShown)
	return found, err
}
