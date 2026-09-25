package e2e

import (
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestZvukToNavidrome(t *testing.T) {
	t.Parallel()

	t.Run("liked tracks are starred after Import", func(t *testing.T) {
		s := newScenario(t)
		account := s.linkNewAccount(alice)
		s.connectZvuk(alice, zvukToken)
		s.addZvukCollection(zvukToken)

		s.importZvuk(alice)
		s.waitIngest()

		env.navidrome.untilStarred(t, account, []string{
			"Liked/Song 1", "Liked/Song 2", "Liked/Song 3", "Liked/Song 4", "Liked/Song 5",
		})
	})

	t.Run("stars follow the order of likes", func(t *testing.T) {
		s := newScenario(t)
		account := s.linkNewAccount(alice)
		s.connectZvuk(alice, zvukToken)
		album := s.addZvukAlbum("730", "Order", 4)
		s.zvuk.update(zvukToken, func(a *zvukAccount) {
			a.liked = []string{album[2], album[0], album[3], album[1]}
		})

		s.importZvuk(alice)
		s.waitIngest()

		env.navidrome.untilStarredByDate(t, account, []string{"Order/Song 3", "Order/Song 1", "Order/Song 4", "Order/Song 2"})
	})

	t.Run("playlist keeps its name and order", func(t *testing.T) {
		s := newScenario(t)
		account := s.linkNewAccount(alice)
		s.connectZvuk(alice, zvukToken)
		album := s.addZvukAlbum("660", "Ordered", 3)
		s.zvuk.addPlaylist(zvukPlaylist{ID: "820", Title: "Road Trip", Tracks: []string{album[2], album[0], album[1]}})
		s.zvuk.update(zvukToken, func(a *zvukAccount) { a.playlists = []string{"820"} })

		s.importZvuk(alice)
		s.waitIngest()

		env.navidrome.untilPlaylist(t, account, "Road Trip", []string{"Ordered/Song 3", "Ordered/Song 1", "Ordered/Song 2"})
	})

	t.Run("stars wait for Navidrome to index the files", func(t *testing.T) {
		s := newScenario(t)
		account := s.linkNewAccount(alice)
		s.connectZvuk(alice, zvukToken)
		s.addZvukCollection(zvukToken)

		s.importZvuk(alice)
		s.waitIngest()

		liked := s.personalPath(alice, "Zvuk Band/Liked (2020)")
		require.Never(t, func() bool {
			starred := len(env.navidrome.starred(t, account)) > 0
			return starred && len(env.navidrome.searchFor(t, account, liked, "Song")) < 5
		}, 300*time.Millisecond, 50*time.Millisecond)
	})

	t.Run("stars and playlists appear once Navidrome indexes the files", func(t *testing.T) {
		s := newScenario(t)
		account := s.linkNewAccount(alice)
		s.connectZvuk(alice, zvukToken)
		s.addZvukCollection(zvukToken)
		s.importZvuk(alice)
		s.waitIngest()

		env.navidrome.scan(t)

		env.navidrome.untilStarred(t, account, []string{
			"Liked/Song 1", "Liked/Song 2", "Liked/Song 3", "Liked/Song 4", "Liked/Song 5",
		})
		env.navidrome.untilPlaylist(t, account, "My Playlist", []string{"Listed/Song 1", "Liked/Song 1"})
	})
}

type subsonicEntry struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Album   string `json:"album"`
	Starred string `json:"starred"`
}

func (e subsonicEntry) name() string {
	return e.Album + "/" + e.Title
}

func (n *navidrome) scan(t *testing.T) {
	t.Helper()
	require.NoError(t, n.subsonic("startScan", nil, nil))
}

// starred lists "album/title" of the songs the account has starred.
func (n *navidrome) starred(t *testing.T, account navidromeAccount) []string {
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

func (n *navidrome) untilStarred(t *testing.T, account navidromeAccount, names []string) {
	t.Helper()

	var got []string
	require.Eventually(t, func() bool {
		_ = n.subsonic("startScan", nil, nil)
		got = n.starred(t, account)
		return slices.Equal(got, names)
	}, time.Minute, 200*time.Millisecond, "starred: %v", &got)
}

// untilStarredByDate rescans until the account's starred songs, newest
// star first as Navidrome's UI sorts them, are exactly these.
func (n *navidrome) untilStarredByDate(t *testing.T, account navidromeAccount, names []string) {
	t.Helper()

	var got []string
	require.Eventually(t, func() bool {
		_ = n.subsonic("startScan", nil, nil)
		got = n.starredByDate(t, account)
		return slices.Equal(got, names)
	}, time.Minute, 200*time.Millisecond, "starred: %v", &got)
}

func (n *navidrome) starredByDate(t *testing.T, account navidromeAccount) []string {
	t.Helper()

	token, err := n.login(account)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodGet, n.url+`/api/song?_sort=starred_at&_order=DESC&_start=0&_end=100&starred=true`, nil)
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

// playlist lists "album/title" of the playlist's songs in order; nil if
// the account has no playlist by that name.
type subsonicPlaylist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (n *navidrome) playlists(t *testing.T, account navidromeAccount) []subsonicPlaylist {
	t.Helper()

	var result struct {
		Playlists struct {
			Playlist []subsonicPlaylist `json:"playlist"`
		} `json:"playlists"`
	}
	require.NoError(t, n.subsonicAs(account, "getPlaylists", nil, &result))
	return result.Playlists.Playlist
}

func (n *navidrome) playlist(t *testing.T, account navidromeAccount, name string) []string {
	t.Helper()

	for _, p := range n.playlists(t, account) {
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

func (n *navidrome) untilPlaylist(t *testing.T, account navidromeAccount, name string, songs []string) {
	t.Helper()

	var got []string
	require.Eventually(t, func() bool {
		_ = n.subsonic("startScan", nil, nil)
		got = n.playlist(t, account, name)
		return slices.Equal(got, songs)
	}, time.Minute, 200*time.Millisecond, "playlist %s: %v", name, &got)
}
