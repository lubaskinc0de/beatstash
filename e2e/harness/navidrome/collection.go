package navidrome

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type subsonicEntry struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Album   string `json:"album"`
	Starred string `json:"starred"`
}

func (e subsonicEntry) name() string {
	return e.Album + "/" + e.Title
}

// Starred lists "album/title" of the songs the account has Starred.
func (n *Server) Starred(t *testing.T, account Account) []string {
	t.Helper()

	var result struct {
		Starred2 struct {
			Song []subsonicEntry `json:"song"`
		} `json:"starred2"`
	}
	require.NoError(t, n.subsonicAs(account, "getStarred2", nil, &result))
	var names []string
	for _, song := range result.Starred2.Song {
		names = append(names, song.name())
	}
	slices.Sort(names)
	return names
}

func (n *Server) UntilStarred(t *testing.T, account Account, names []string) {
	t.Helper()

	var got []string
	require.Eventually(t, func() bool {
		_ = n.scanFor(account)
		got = n.Starred(t, account)
		return slices.Equal(got, names)
	}, time.Minute, 200*time.Millisecond, "starred: %v", &got)
}

// UntilStarredByDate rescans until the account's starred songs, newest
// star first as Navidrome's UI sorts them, are exactly these.
func (n *Server) UntilStarredByDate(t *testing.T, account Account, names []string) {
	t.Helper()

	var got []string
	require.Eventually(t, func() bool {
		_ = n.scanFor(account)
		got = n.starredByDate(t, account)
		return slices.Equal(got, names)
	}, time.Minute, 200*time.Millisecond, "starred: %v", &got)
}

func (n *Server) starredByDate(t *testing.T, account Account) []string {
	t.Helper()

	token, err := n.login(account)
	require.NoError(t, err)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, n.URL+`/api/song?_sort=starred_at&_order=DESC&_start=0&_end=100&starred=true`, nil)
	require.NoError(t, err)
	req.Header.Set("X-Nd-Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	var songs []struct {
		Title string `json:"title"`
		Album string `json:"album"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&songs))
	var names []string
	for _, song := range songs {
		names = append(names, song.Album+"/"+song.Title)
	}
	return names
}

type Playlist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (n *Server) Playlists(t *testing.T, account Account) []Playlist {
	t.Helper()

	var result struct {
		Playlists struct {
			Playlist []Playlist `json:"playlist"`
		} `json:"playlists"`
	}
	require.NoError(t, n.subsonicAs(account, "getPlaylists", nil, &result))
	return result.Playlists.Playlist
}

// playlist lists "album/title" of the playlist's songs in order; nil if
// the account has no playlist by that name.
func (n *Server) playlist(t *testing.T, account Account, name string) []string {
	t.Helper()

	for _, p := range n.Playlists(t, account) {
		if p.Name != name {
			continue
		}
		var playlist struct {
			Playlist struct {
				Entry []subsonicEntry `json:"entry"`
			} `json:"playlist"`
		}
		require.NoError(t, n.subsonicAs(account, "getPlaylist", url.Values{"id": {p.ID}}, &playlist))
		names := []string{}
		for _, song := range playlist.Playlist.Entry {
			names = append(names, song.name())
		}
		return names
	}
	return nil
}

func (n *Server) UntilPlaylist(t *testing.T, account Account, name string, songs []string) {
	t.Helper()

	var got []string
	require.Eventually(t, func() bool {
		_ = n.scanFor(account)
		got = n.playlist(t, account, name)
		return slices.Equal(got, songs)
	}, time.Minute, 200*time.Millisecond, "playlist %s: %v", name, &got)
}
