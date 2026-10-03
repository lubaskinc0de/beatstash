package providers

import (
	"github.com/lubaskinc0de/beatstash/internal/domain/provider"
)

type ListedTrack struct {
	Ref         provider.TrackRef
	DisplayName string
	Bytes       int64
}

type ListedPlaylist struct {
	ID     string
	Title  string
	Tracks []ListedTrack
}

type Collection struct {
	// LikedRefs lists every like, even of a track the Provider no longer
	// serves: Liked has only the tracks it does, and a like missing there
	// would take the star off.
	LikedRefs   []string
	Liked       []ListedTrack
	AlbumTracks []ListedTrack
	Playlists   []ListedPlaylist
}

func (c *Collection) Tracks() []ListedTrack {
	seen := map[string]bool{}
	var tracks []ListedTrack
	add := func(list []ListedTrack) {
		for _, track := range list {
			if !seen[track.Ref.ID] {
				seen[track.Ref.ID] = true
				tracks = append(tracks, track)
			}
		}
	}
	add(c.Liked)
	add(c.AlbumTracks)
	for _, playlist := range c.Playlists {
		add(playlist.Tracks)
	}
	return tracks
}

func (c *Collection) Snapshot() *provider.CollectionSnapshot {
	var playlists []provider.PlaylistSnapshot
	for _, playlist := range c.Playlists {
		playlists = append(playlists, provider.PlaylistSnapshot{
			ID:     playlist.ID,
			Title:  playlist.Title,
			Tracks: RefIDs(playlist.Tracks),
		})
	}
	return &provider.CollectionSnapshot{
		Tracks:    RefIDs(c.Tracks()),
		Liked:     c.LikedRefs,
		Playlists: playlists,
	}
}

func RefIDs(tracks []ListedTrack) []string {
	ids := make([]string, 0, len(tracks))
	for _, track := range tracks {
		ids = append(ids, track.Ref.ID)
	}
	return ids
}

func RefSet(items []string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, item := range items {
		set[item] = true
	}
	return set
}
