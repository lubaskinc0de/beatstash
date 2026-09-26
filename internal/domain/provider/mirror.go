package provider

import (
	"maps"
	"slices"
)

// MirrorState is a value object: the stars and playlists the bot has created
// in Navidrome for a Provider Collection. The bot changes only these.
type MirrorState struct {
	// Starred maps liked tracks to the songs the bot has starred for them.
	Starred   map[string]string           `json:"starred,omitempty"`
	Playlists map[string]MirroredPlaylist `json:"playlists,omitempty"`
}

// MirroredPlaylist is a value object: a playlist the bot created in Navidrome.
type MirroredPlaylist struct {
	NavidromeID string `json:"navidrome_id"`
	// Songs are the Navidrome song ids the bot last put in the playlist.
	Songs []string `json:"songs"`
}

// WantedMirror is a value object: the MirrorState the bot should reach. It
// also keeps the order of the changes: stars from the oldest like, playlists
// in the Provider's order.
type WantedMirror struct {
	MirrorState

	starOrder []string
	playlists []PlaylistSnapshot
}

// MirrorChange is a value object: the calls to Navidrome that turn one
// MirrorState into another.
type MirrorChange struct {
	Star      []Star
	Unstar    []Star
	Playlists []PlaylistChange
}

// Star is a value object: a liked track and its Navidrome song.
type Star struct {
	Ref  string
	Song string
}

// PlaylistChange is a value object: set the playlist's songs. An empty
// NavidromeID means the playlist must be created.
type PlaylistChange struct {
	ID          string
	NavidromeID string
	Title       string
	Songs       []string
}

func (w WantedMirror) ChangesFrom(current MirrorState) MirrorChange {
	var change MirrorChange
	for _, ref := range w.starOrder {
		song, ok := w.Starred[ref]
		if ok && current.Starred[ref] != song {
			change.Star = append(change.Star, Star{Ref: ref, Song: song})
		}
	}
	for ref, song := range current.Starred {
		if _, ok := w.Starred[ref]; !ok {
			change.Unstar = append(change.Unstar, Star{Ref: ref, Song: song})
		}
	}
	for _, playlist := range w.playlists {
		wanted, ok := w.Playlists[playlist.ID]
		mirrored, exists := current.Playlists[playlist.ID]
		if !ok || exists && slices.Equal(mirrored.Songs, wanted.Songs) {
			continue
		}
		change.Playlists = append(change.Playlists, PlaylistChange{
			ID:          playlist.ID,
			NavidromeID: mirrored.NavidromeID,
			Title:       playlist.Title,
			Songs:       wanted.Songs,
		})
	}
	return change
}

// Apply returns the state after the change. If the change stopped halfway,
// pass only the part that was done.
func (s MirrorState) Apply(done MirrorChange) MirrorState {
	next := MirrorState{Starred: maps.Clone(s.Starred), Playlists: maps.Clone(s.Playlists)}
	if next.Starred == nil {
		next.Starred = map[string]string{}
	}
	if next.Playlists == nil {
		next.Playlists = map[string]MirroredPlaylist{}
	}
	for _, star := range done.Star {
		next.Starred[star.Ref] = star.Song
	}
	for _, star := range done.Unstar {
		delete(next.Starred, star.Ref)
	}
	for _, playlist := range done.Playlists {
		next.Playlists[playlist.ID] = MirroredPlaylist{NavidromeID: playlist.NavidromeID, Songs: playlist.Songs}
	}
	return next
}
