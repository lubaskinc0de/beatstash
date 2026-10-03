package send_listen_link

import (
	"context"
	"time"

	"github.com/lubaskinc0de/beatstash/internal/application/common"
	"github.com/lubaskinc0de/beatstash/internal/application/common/libraries"
	"github.com/lubaskinc0de/beatstash/internal/application/common/listening"
	"github.com/lubaskinc0de/beatstash/internal/application/common/navidrome"
	"github.com/lubaskinc0de/beatstash/internal/application/common/repositories"
	"github.com/lubaskinc0de/beatstash/internal/domain/library"
)

type TrackLink struct {
	Track *library.Track
	URL   string
}

// GetTrackListenLink reuses the user's link while it lasts. Once the Track
// is loaded, a TrackLink names it even with an error.
type GetTrackListenLink struct {
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

func (i *GetTrackListenLink) Execute(ctx context.Context, trackID uint) (*TrackLink, error) {
	user, track, err := audible(ctx, i.IDs, i.Libraries, i.Tracks, trackID)
	if err != nil {
		return nil, err
	}
	found := &TrackLink{Track: track}
	creds, err := i.ListenLinks.Credentials(ctx, user.ID)
	if err != nil {
		return found, err
	}
	songID, err := songOf(ctx, i.Tracks, i.LibraryRepo, i.Navidrome, i.Admin, track)
	if err != nil {
		return found, err
	}
	link, err := i.Links.OfTrack(ctx, user.ID, track.ID)
	if err != nil {
		return found, err
	}
	if link != nil && link.ServesTrack(track, i.Clock()) {
		found.URL = link.URL
		return found, nil
	}
	expires := i.Clock().Add(i.TTL)
	url, err := i.Navidrome.CreateShare(ctx, creds, navidrome.Share{
		ID: songID, Description: description(track.Artist, track.Title), Expires: expires, Downloadable: i.Downloadable,
	})
	if err != nil {
		return found, err
	}
	if err := i.Links.Save(ctx, library.NewTrackListenLink(user.ID, track, url, expires)); err != nil {
		return found, err
	}
	found.URL = url
	return found, nil
}
