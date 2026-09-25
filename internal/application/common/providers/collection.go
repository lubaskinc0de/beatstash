package providers

import "github.com/lubaskinc0de/navidrome-tg/internal/domain"

type ListedTrack struct {
	Ref         domain.TrackRef
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

func (c *Collection) Snapshot() *domain.CollectionSnapshot {
	snapshot := &domain.CollectionSnapshot{
		Tracks: RefIDs(c.Tracks()),
		Liked:  c.LikedRefs,
	}
	for _, playlist := range c.Playlists {
		snapshot.Playlists = append(snapshot.Playlists, domain.PlaylistSnapshot{
			ID:     playlist.ID,
			Title:  playlist.Title,
			Tracks: RefIDs(playlist.Tracks),
		})
	}
	return snapshot
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
