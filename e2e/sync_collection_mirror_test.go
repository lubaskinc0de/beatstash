package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lubaskinc0de/beatstash/e2e/harness"
	"github.com/lubaskinc0de/beatstash/e2e/harness/zvuk"
)

func TestZvukToNavidrome(t *testing.T) {
	t.Parallel()

	t.Run("liked tracks are starred after Import", func(t *testing.T) {
		s := harness.New(t)
		account := s.LinkNewAccount(alice)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)

		s.ImportZvuk(alice)
		s.WaitIngest()

		s.Navidrome.UntilStarred(t, account, []string{
			"Liked/Song 1", "Liked/Song 2", "Liked/Song 3", "Liked/Song 4", "Liked/Song 5",
		})
	})

	t.Run("stars follow the order of likes", func(t *testing.T) {
		s := harness.New(t)
		account := s.LinkNewAccount(alice)
		s.ConnectZvuk(alice, harness.ZvukToken)
		album := s.AddZvukAlbum("730", "Order", 4)
		s.Zvuk.Update(harness.ZvukToken, func(a *zvuk.Account) {
			a.Liked = []string{album[2], album[0], album[3], album[1]}
		})

		s.ImportZvuk(alice)
		s.WaitIngest()

		s.Navidrome.UntilStarredByDate(t, account, []string{"Order/Song 3", "Order/Song 1", "Order/Song 4", "Order/Song 2"})
	})

	t.Run("playlist keeps its name and order", func(t *testing.T) {
		s := harness.New(t)
		account := s.LinkNewAccount(alice)
		s.ConnectZvuk(alice, harness.ZvukToken)
		album := s.AddZvukAlbum("660", "Ordered", 3)
		s.Zvuk.AddPlaylist(zvuk.Playlist{ID: "820", Title: "Road Trip", Tracks: []string{album[2], album[0], album[1]}})
		s.Zvuk.Update(harness.ZvukToken, func(a *zvuk.Account) { a.Playlists = []string{"820"} })

		s.ImportZvuk(alice)
		s.WaitIngest()

		s.Navidrome.UntilPlaylist(t, account, "Road Trip", []string{"Ordered/Song 3", "Ordered/Song 1", "Ordered/Song 2"})
	})

	t.Run("stars wait for Navidrome to index the files", func(t *testing.T) {
		s := harness.New(t)
		account := s.LinkNewAccount(alice)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.ImportZvuk(alice)
		s.WaitIngest()

		s.RunSync()

		starred := len(s.Navidrome.Starred(t, account)) > 0
		indexed := len(s.Navidrome.SearchFor(t, account, s.PersonalPath(alice, "Zvuk Band/Liked (2020)"), "Song")) == 5
		assert.True(t, !starred || indexed, "stars came before the songs were indexed")
	})

	t.Run("stars and playlists appear once Navidrome indexes the files", func(t *testing.T) {
		s := harness.New(t)
		account := s.LinkNewAccount(alice)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.ImportZvuk(alice)
		s.WaitIngest()

		s.Navidrome.Scan(t, account)

		s.Navidrome.UntilStarred(t, account, []string{
			"Liked/Song 1", "Liked/Song 2", "Liked/Song 3", "Liked/Song 4", "Liked/Song 5",
		})
		s.Navidrome.UntilPlaylist(t, account, "My Playlist", []string{"Listed/Song 1", "Liked/Song 1"})
	})
}
