package harness

import (
	"path/filepath"
	"regexp"
	"slices"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/navidrome"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/telegram"
)

func (s *Scenario) LinkNewAccount(user User) navidrome.Account {
	s.t.Helper()

	account := s.Navidrome.CreateAccount(s.t, user.Username)
	s.Link(user, account)
	require.Contains(s.t, s.WindowText(), "привязан")
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
	s.t.Helper()

	s.Open(from, "👤 Аккаунты")
	link := "🔗 Привязать"
	if !slices.Contains(telegram.ButtonTexts(s.Telegram.Buttons(s.t)), link) {
		link = "🔗 Привязать другой"
	}
	s.Go(from, link)
	return s.SendText(from, account.Login+" "+account.Password)
}

func (s *Scenario) SendText(from User, text string) *models.Update {
	msg := s.TextMessage(from, text)
	s.Send(msg)
	return msg
}

// Register uses the newcomer's username as the Navidrome login.
func (s *Scenario) Register(newcomer User) navidrome.Account {
	s.t.Helper()

	s.Send(s.TextMessage(newcomer, "/start "+s.Invite()))
	s.SendText(newcomer, newcomer.Username)
	return s.IssuedAccount()
}
