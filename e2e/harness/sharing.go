package harness

import (
	"slices"

	"github.com/go-telegram/bot/models"
)

// Share answers the upload with /Share and presses the named button.
func (s *Scenario) Share(from User, upload *models.Update, name string) {
	s.t.Helper()

	s.Send(s.ReplyCommand(from, "/share", upload))
	s.Press(from, s.Button(name))
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

func (s *Scenario) ShareOnScreen(user User, track string) {
	s.t.Helper()

	c := s.Catalog(user)
	s.Open(user, c.MusicButton(), c.MineTab(), track, c.ShareCardButton())
}
