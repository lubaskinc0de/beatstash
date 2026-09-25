package harness

import (
	"strconv"

	"github.com/go-telegram/bot/models"
)

// Share answers the upload with /Share and presses the named button.
func (s *Scenario) Share(from User, upload *models.Update, name string) {
	s.t.Helper()

	s.Send(s.ReplyCommand(from, "/share", upload))
	s.Press(from, s.Button(name))
}

// Take presses "Take" on the n-th Share of the feed.
func (s *Scenario) Take(user User, n int) {
	s.t.Helper()

	s.Send(s.TextMessage(user, "/shared"))
	s.Press(user, s.Button(strconv.Itoa(n)+". ➕ Взять себе"))
}
