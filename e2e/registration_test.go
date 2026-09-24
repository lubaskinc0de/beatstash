package e2e

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistration(t *testing.T) {
	t.Run("invited person gets a Navidrome account named after their username", func(t *testing.T) {
		s := newScenario(t)
		carol := newcomer("carol")
		code := s.invite()

		s.send(s.textMessage(carol, "/start "+code))

		account := issuedAccount(t, s)
		assert.Equal(t, carol.Username, account.Login)
		assert.True(t, env.navidrome.canLogin(account))
	})

	t.Run("person without username chooses a login", func(t *testing.T) {
		s := newScenario(t)
		carol := newcomer("carol")
		login := carol.Username
		carol.Username = ""
		s.send(s.textMessage(carol, "/start "+s.invite()))
		assert.Contains(t, lastReply(t, s).Text, "Придумайте логин")

		s.send(s.textMessage(carol, login))

		account := issuedAccount(t, s)
		assert.Equal(t, login, account.Login)
		assert.True(t, env.navidrome.canLogin(account))
	})

	t.Run("taken username makes the bot ask for another login", func(t *testing.T) {
		s := newScenario(t)
		taken := env.navidrome.createAccount(t, "carol")
		carol := newcomer("carol")
		carol.Username = taken.Login
		s.send(s.textMessage(carol, "/start "+s.invite()))
		assert.Contains(t, lastReply(t, s).Text, "уже занят")
		login := uniqueLogin("carol")

		s.send(s.textMessage(carol, login))

		account := issuedAccount(t, s)
		assert.Equal(t, login, account.Login)
		assert.True(t, env.navidrome.canLogin(account))
		assert.True(t, env.navidrome.canLogin(taken))
	})

	t.Run("login answer after restart is processed", func(t *testing.T) {
		s := newScenario(t)
		carol := newcomer("carol")
		login := carol.Username
		carol.Username = ""
		s.send(s.textMessage(carol, "/start "+s.invite()))

		s.restart()
		s.send(s.textMessage(carol, login))

		account := issuedAccount(t, s)
		assert.Equal(t, login, account.Login)
		assert.True(t, env.navidrome.canLogin(account))
	})

	t.Run("linking an existing account ends the login dialog", func(t *testing.T) {
		s := newScenario(t)
		carol := newcomer("carol")
		carol.Username = ""
		s.send(s.textMessage(carol, "/start "+s.invite()))
		s.link(carol, env.navidrome.createAccount(t, "carol"))
		replies := len(s.botAPI.Replies(t))

		s.send(s.textMessage(carol, uniqueLogin("carol")))

		assert.Len(t, s.botAPI.Replies(t), replies)
	})

	t.Run("registered user sees their np", func(t *testing.T) {
		s := newScenario(t)
		carol := newcomer("carol")
		s.send(s.textMessage(carol, "/start "+s.invite()))
		account := issuedAccount(t, s)
		audio := s.uploadAudio("track.mp3")
		s.send(s.audioMessage(carol, audio))
		env.navidrome.startPlaying(t, account, env.navidrome.indexedTrack(t, account, s.library, fixtureTitle).ID)
		query := s.inlineQuery(carol, "np")

		s.send(query)

		answer := s.botAPI.InlineAnswerTo(t, query)
		require.Len(t, answer.Results, 1)
		assert.Equal(t, audio.FileID, answer.Results[0].AudioFileID)
	})
}

var (
	issuedLogin    = regexp.MustCompile(`Логин: <code>([^<]+)</code>`)
	issuedPassword = regexp.MustCompile(`<tg-spoiler>([^<]+)</tg-spoiler>`)
)

// issuedAccount reads the credentials the bot showed in its last reply.
func issuedAccount(t *testing.T, s *scenario) navidromeAccount {
	t.Helper()

	text := lastReply(t, s).Text
	login := issuedLogin.FindStringSubmatch(text)
	password := issuedPassword.FindStringSubmatch(text)
	require.NotNil(t, login, "no login in %q", text)
	require.NotNil(t, password, "no hidden password in %q", text)
	return navidromeAccount{Login: login[1], Password: password[1]}
}
