package send_listen_link

import (
	"context"
	"errors"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/listening"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/navidrome"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

var ErrSingle = errors.New("the track belongs to no album")

type AlbumLink struct {
	Album library.AlbumKey
	URL   string
}

// GetAlbumListenLink links to the Navidrome album of an indexed Track:
// songs indexed later show up there too.
type GetAlbumListenLink struct {
	IDs         common.IDProvider
	Tracks      repositories.Tracks
	Links       repositories.ListenLinks
	LibraryRepo repositories.Libraries
	Libraries   *libraries.Libraries
	Attached    *libraries.Attached
	ListenLinks *listening.ListenLinks
	Navidrome   navidrome.Client
	// Admin finds the songs of any library.
	Admin        navidrome.Credentials
	TTL          time.Duration
	Downloadable bool
	Clock        func() time.Time
}

func (i *GetAlbumListenLink) Execute(ctx context.Context, trackID uint) (*AlbumLink, error) {
	user, track, err := audible(ctx, i.IDs, i.Libraries, i.Attached, i.Tracks, trackID)
	if err != nil {
		return nil, err
	}
	found := &AlbumLink{Album: track.AlbumKey()}
	creds, err := i.ListenLinks.Credentials(ctx, user.ID)
	if err != nil {
		return found, err
	}
	if track.Single() {
		return found, ErrSingle
	}
	link, err := i.Links.OfAlbum(ctx, user.ID, found.Album)
	if err != nil {
		return found, err
	}
	if link != nil && link.Valid(i.Clock()) {
		found.URL = link.URL
		return found, nil
	}
	songID, err := i.anySong(ctx, found.Album, track)
	if err != nil {
		return found, err
	}
	albumID, err := i.Navidrome.AlbumOf(ctx, creds, songID)
	if err != nil {
		return found, err
	}
	expires := i.Clock().Add(i.TTL)
	url, err := i.Navidrome.CreateShare(ctx, creds, navidrome.Share{
		ID: albumID, Description: description(found.Album.AlbumArtist, found.Album.Album), Expires: expires, Downloadable: i.Downloadable,
	})
	if err != nil {
		return found, err
	}
	if err := i.Links.Save(ctx, library.NewAlbumListenLink(user.ID, found.Album, url, expires)); err != nil {
		return found, err
	}
	found.URL = url
	return found, nil
}

// anySong falls back to asking Navidrome for the chosen Track.
func (i *GetAlbumListenLink) anySong(ctx context.Context, album library.AlbumKey, chosen *library.Track) (string, error) {
	tracks, err := i.Tracks.Album(ctx, album)
	if err != nil {
		return "", err
	}
	for _, t := range tracks {
		if t.Indexed() {
			return t.SongID, nil
		}
	}
	return songOf(ctx, i.Tracks, i.LibraryRepo, i.Navidrome, i.Admin, chosen)
}
