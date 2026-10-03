package harness

import (
	"slices"

	"github.com/lubaskinc0de/beatstash/internal/application/common/repositories"
)

// ShareTrack shares the user's track from its card; track is its button,
// "Artist — Title".
func (s *Scenario) ShareTrack(user User, track string) {
	s.t.Helper()

	c := s.Catalog(user)
	s.Open(user, c.MusicButton(), c.MineTab(), track, c.ShareCardButton())
}

func (s *Scenario) UnshareTrack(user User, track string) {
	s.t.Helper()

	c := s.Catalog(user)
	s.Open(user, c.MusicButton(), c.MineTab(), track, c.UnshareCardButton())
}

func (s *Scenario) ShareAlbum(user User, album repositories.AlbumSummary) {
	s.t.Helper()

	c := s.Catalog(user)
	s.Open(user, c.MusicButton(), c.MineTab(), c.AlbumsMode(), c.OwnAlbumButton(album), c.ShareAlbumButton())
}

func (s *Scenario) UnshareAlbum(user User, album repositories.AlbumSummary) {
	s.t.Helper()

	c := s.Catalog(user)
	s.Open(user, c.MusicButton(), c.MineTab(), c.AlbumsMode(), c.OwnAlbumButton(album), c.UnshareAlbumButton())
}

// Take presses "Take" on the card of the n-th Share of the feed.
func (s *Scenario) Take(user User, n int) {
	s.t.Helper()

	s.OpenShared(user, n)
	s.Go(user, s.Catalog(user).TakeButton())
}

// OpenShared opens the card of the n-th Share of the feed, counted from one.
func (s *Scenario) OpenShared(user User, n int) {
	s.t.Helper()

	c := s.Catalog(user)
	s.Open(user, c.MusicButton())
	nav := []string{c.OpenTab(c.SharedTab()), c.MineTab(), c.TopButton(), c.Back()}
	left := n
	for _, b := range s.Telegram.Buttons(s.t) {
		if slices.Contains(nav, b.Text) {
			continue
		}
		if left--; left == 0 {
			s.Press(user, b)
			return
		}
	}
	s.t.Fatalf("the feed has no Share #%d", n)
}
