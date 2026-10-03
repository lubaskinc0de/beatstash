package send_listen_link

import (
	"context"
	"errors"
	"time"

	"github.com/lubaskinc0de/beatstash/internal/application/common"
	"github.com/lubaskinc0de/beatstash/internal/application/common/libraries"
	"github.com/lubaskinc0de/beatstash/internal/application/common/listening"
	"github.com/lubaskinc0de/beatstash/internal/application/common/navidrome"
	"github.com/lubaskinc0de/beatstash/internal/application/common/repositories"
	"github.com/lubaskinc0de/beatstash/internal/domain/library"
)

var ErrSingle = errors.New("the track belongs to no album")

// AlbumLink's Album has only its key for a single.
type AlbumLink struct {
	Album repositories.AlbumSummary
	URL   string
}

// GetAlbumListenLink links to the Navidrome album of an indexed Track:
// songs indexed later show up there too.
type GetAlbumListenLink struct {
	IDs         common.IDProvider
	Tracks      repositories.Tracks
	Links       repositories.ListenLinks
	Libraries   *libraries.Libraries
	LibraryRepo repositories.Libraries
	ListenLinks *listening.ListenLinks
	Navidrome   navidrome.Client
	// Admin finds the songs of any library.
	Admin        navidrome.Credentials
	TTL          time.Duration
	Downloadable bool
	Clock        func() time.Time
}

func (i *GetAlbumListenLink) Execute(ctx context.Context, trackID uint) (*AlbumLink, error) {
	user, track, err := audible(ctx, i.IDs, i.Libraries, i.Tracks, trackID)
	if err != nil {
		return nil, err
	}
	album := track.AlbumKey()
	found := &AlbumLink{Album: repositories.AlbumSummary{AlbumKey: album}}
	if !track.Single() {
		if found.Album, err = i.Tracks.AlbumSummary(ctx, album); err != nil {
			return nil, err
		}
	}
	creds, err := i.ListenLinks.Credentials(ctx, user.ID)
	if err != nil {
		return found, err
	}
	if track.Single() {
		return found, ErrSingle
	}
	link, err := i.Links.OfAlbum(ctx, user.ID, album)
	if err != nil {
		return found, err
	}
	if link != nil && link.Valid(i.Clock()) {
		found.URL = link.URL
		return found, nil
	}
	songID, err := i.anySong(ctx, album, track)
	if err != nil {
		return found, err
	}
	albumID, err := i.Navidrome.AlbumOf(ctx, creds, songID)
	if err != nil {
		return found, err
	}
	expires := i.Clock().Add(i.TTL)
	url, err := i.Navidrome.CreateShare(ctx, creds, navidrome.Share{
		ID: albumID, Description: description(album.AlbumArtist, album.Album), Expires: expires, Downloadable: i.Downloadable,
	})
	if err != nil {
		return found, err
	}
	if err := i.Links.Save(ctx, library.NewAlbumListenLink(user.ID, album, url, expires)); err != nil {
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
