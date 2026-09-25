package e2e

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

const zvukToken = "zvuk-token-alice"

func TestZvukAccount(t *testing.T) {
	t.Parallel()

	t.Run("valid token connects Zvuk", func(t *testing.T) {
		s := newScenario(t)
		s.zvuk.addAccount(zvukToken, true)
		msg := s.textMessage(alice, "/zvuk "+zvukToken)

		s.send(msg)

		assert.Contains(t, s.botAPI.deletedMessages(), strconv.Itoa(msg.Message.ID))
		assert.Contains(t, lastReply(t, s).Text, "Звук подключён")
	})

	t.Run("invalid token is rejected", func(t *testing.T) {
		s := newScenario(t)
		msg := s.textMessage(alice, "/zvuk wrong-token")

		s.send(msg)

		assert.Contains(t, s.botAPI.deletedMessages(), strconv.Itoa(msg.Message.ID))
		assert.Contains(t, lastReply(t, s).Text, "Звук не принял токен")
	})

	t.Run("rejected token leaves Zvuk unconnected", func(t *testing.T) {
		s := newScenario(t)
		s.send(s.textMessage(alice, "/zvuk wrong-token"))

		s.send(s.textMessage(alice, "/zvuk_import"))

		assert.Contains(t, lastReply(t, s).Text, "Подключите Звук")
	})

	t.Run("account without subscription is rejected", func(t *testing.T) {
		s := newScenario(t)
		s.zvuk.addAccount(zvukToken, false)

		s.send(s.textMessage(alice, "/zvuk "+zvukToken))

		assert.Contains(t, lastReply(t, s).Text, "нет подписки")
	})

	t.Run("account without subscription stays unconnected", func(t *testing.T) {
		s := newScenario(t)
		s.zvuk.addAccount(zvukToken, false)
		s.send(s.textMessage(alice, "/zvuk "+zvukToken))

		s.send(s.textMessage(alice, "/zvuk_import"))

		assert.Contains(t, lastReply(t, s).Text, "Подключите Звук")
	})

	t.Run("user disconnects Zvuk", func(t *testing.T) {
		s := newScenario(t)
		s.connectZvuk(alice, zvukToken)

		s.send(s.textMessage(alice, "/zvuk_off"))

		assert.Contains(t, lastReply(t, s).Text, "Звук отключён")
	})

	t.Run("disconnected account asks to connect again", func(t *testing.T) {
		s := newScenario(t)
		s.connectZvuk(alice, zvukToken)
		s.send(s.textMessage(alice, "/zvuk_off"))

		s.send(s.textMessage(alice, "/zvuk_import"))

		assert.Contains(t, lastReply(t, s).Text, "Подключите Звук")
	})

	t.Run("unavailable Zvuk leaves the account unconnected", func(t *testing.T) {
		s := newScenario(t)
		s.zvuk.addAccount(zvukToken, true)
		s.zvuk.setDown(true)

		s.send(s.textMessage(alice, "/zvuk "+zvukToken))

		assert.Contains(t, lastReply(t, s).Text, "Звук недоступен")
	})
}

// connectZvuk creates the Zvuk account anew, with a subscription and an
// empty collection, and connects it to the bot.
func (s *scenario) connectZvuk(user telegramUser, token string) *zvukAccount {
	s.t.Helper()

	account := s.zvuk.addAccount(token, true)
	s.send(s.textMessage(user, "/zvuk "+token))
	assert.Contains(s.t, lastReply(s.t, s).Text, "Звук подключён")
	return account
}
