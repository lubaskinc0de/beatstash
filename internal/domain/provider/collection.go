package provider

// CollectionSnapshot is a value object: a Provider Collection at the moment
// of the last Import or Sync.
type CollectionSnapshot struct {
	// Tracks lists every track once: liked tracks first, then saved
	// albums, then playlists.
	Tracks    []string           `json:"tracks"`
	Liked     []string           `json:"liked"`
	Playlists []PlaylistSnapshot `json:"playlists"`
}

// PlaylistSnapshot is a value object: a playlist in a CollectionSnapshot.
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
