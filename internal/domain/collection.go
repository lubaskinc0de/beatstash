package domain

type CollectionSnapshot struct {
	// Tracks lists every track once: liked ones, then those of saved
	// albums, then those of playlists.
	Tracks    []string           `json:"tracks"`
	Liked     []string           `json:"liked"`
	Playlists []PlaylistSnapshot `json:"playlists"`
}

type PlaylistSnapshot struct {
	ID     string   `json:"id"`
	Title  string   `json:"title"`
	Tracks []string `json:"tracks"`
}

func (c *CollectionSnapshot) Added(earlier *CollectionSnapshot) []string {
	if earlier == nil {
		return c.Tracks
	}
	known := make(map[string]bool, len(earlier.Tracks))
	for _, id := range earlier.Tracks {
		known[id] = true
	}
	var added []string
	for _, id := range c.Tracks {
		if !known[id] {
			added = append(added, id)
		}
	}
	return added
}

type MirrorState struct {
	// Starred maps liked tracks to the songs the bot has starred for them.
	Starred   map[string]string           `json:"starred,omitempty"`
	Playlists map[string]MirroredPlaylist `json:"playlists,omitempty"`
}

type MirroredPlaylist struct {
	NavidromeID string `json:"navidrome_id"`
	// Songs are the Navidrome song ids the playlist got last time.
	Songs []string `json:"songs"`
}
