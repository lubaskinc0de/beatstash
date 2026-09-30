package harness

import "github.com/go-telegram/bot/models"

// Share answers the upload with /Share and presses the named button.
func (s *Scenario) Share(from User, upload *models.Update, name string) {
	s.t.Helper()

	s.Send(s.ReplyCommand(from, "/share", upload))
	s.Press(from, s.Button(name))
}

// Take presses "Take" on the n-th Share of the feed.
func (s *Scenario) Take(user User, n int) {
	s.t.Helper()

	s.Open(user, s.Catalog(user).FeedButton(), s.Catalog(user).TakeButton(n))
}
