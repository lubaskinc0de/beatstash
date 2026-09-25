package harness

import (
	"path/filepath"
	"regexp"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/navidrome"
)

func (s *Scenario) LinkNewAccount(user User) navidrome.Account {
	s.t.Helper()

	account := s.Navidrome.CreateAccount(s.t, user.Username)
	s.Link(user, account)
	require.Contains(s.t, s.LastReply().Text, "привязан")
	return account
}

func (s *Scenario) NavidromePath(rel string) string {
	return filepath.Join(s.config.NavidromeMusicDir, rel)
}

var (
	issuedLogin    = regexp.MustCompile(`Логин: <code>([^<]+)</code>`)
	issuedPassword = regexp.MustCompile(`<tg-spoiler>([^<]+)</tg-spoiler>`)
)

// IssuedAccount reads the credentials the bot showed in its last reply.
func (s *Scenario) IssuedAccount() navidrome.Account {
	s.t.Helper()

	text := s.LastReply().Text
	login := issuedLogin.FindStringSubmatch(text)
	password := issuedPassword.FindStringSubmatch(text)
	require.NotNil(s.t, login, "no login in %q", text)
	require.NotNil(s.t, password, "no hidden password in %q", text)
	return navidrome.Account{Login: login[1], Password: password[1]}
}

func (s *Scenario) Link(from User, account navidrome.Account) *models.Update {
	msg := s.TextMessage(from, "/link "+account.Login+" "+account.Password)
	s.Send(msg)
	return msg
}
