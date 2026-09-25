package e2e

import (
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvite(t *testing.T) {
	t.Run("invited person becomes a user", func(t *testing.T) {
		s := newScenario(t)
		carol := newcomer("carol")
		code := s.invite()

		s.send(s.textMessage(carol, "/start "+code))
		audio := s.uploadAudio("track.mp3")
		upload := s.audioMessage(carol, audio)
		s.send(upload)
		s.waitIngest()

		assert.Contains(t, s.botAPI.replies(t)[1].Text, "Добро пожаловать")
		assert.Equal(t, []string{"👀", "👍"}, s.botAPI.reactionsOn(t, upload.Message.ID))
	})

	t.Run("used invite is rejected", func(t *testing.T) {
		s := newScenario(t)
		code := s.invite()
		s.send(s.textMessage(newcomer("carol"), "/start "+code))
		dave := newcomer("dave")

		s.send(s.textMessage(dave, "/start "+code))
		upload := s.audioMessage(dave, s.uploadAudio("track.mp3"))
		s.send(upload)

		assert.Contains(t, lastReply(t, s).Text, "Приглашение недействительно")
		assert.Empty(t, s.botAPI.reactionsOn(t, upload.Message.ID))
	})

	t.Run("invite expires after 7 days", func(t *testing.T) {
		s := newScenario(t)
		code := s.invite()
		s.clock.advance(7*24*time.Hour + time.Minute)
		carol := newcomer("carol")

		s.send(s.textMessage(carol, "/start "+code))
		upload := s.audioMessage(carol, s.uploadAudio("track.mp3"))
		s.send(upload)

		assert.Contains(t, lastReply(t, s).Text, "Приглашение недействительно")
		assert.Empty(t, s.botAPI.reactionsOn(t, upload.Message.ID))
	})

	t.Run("non-admin cannot invite", func(t *testing.T) {
		s := newScenario(t)

		s.send(s.textMessage(alice, "/invite"))

		replies := s.botAPI.replies(t)
		require.Len(t, replies, 1)
		assert.Contains(t, replies[0].Text, "только администратор")
		assert.NotContains(t, replies[0].Text, "?start=")
	})

	t.Run("stranger gets no answer but to start", func(t *testing.T) {
		s := newScenario(t)

		s.send(s.textMessage(stranger, "/invite"))
		s.send(s.textMessage(stranger, "/share"))
		s.send(s.textMessage(stranger, "/top"))
		s.send(s.textMessage(stranger, "hello"))
		s.send(s.audioMessage(stranger, s.uploadAudio("track.mp3")))

		assert.Empty(t, s.botAPI.allCalls())
	})
}

var inviteLink = regexp.MustCompile(`https://t\.me/` + botUsername + `\?start=([A-Za-z0-9_-]+)`)

func (s *scenario) invite() string {
	s.t.Helper()

	s.send(s.textMessage(adminUser, "/invite"))
	match := inviteLink.FindStringSubmatch(lastReply(s.t, s).Text)
	require.NotNil(s.t, match, "no invite link in the reply")
	return match[1]
}

func TestStart(t *testing.T) {
	t.Run("user gets a welcome with instructions", func(t *testing.T) {
		s := newScenario(t)

		s.send(s.textMessage(alice, "/start"))

		text := lastReply(t, s).Text
		assert.Contains(t, text, "Добро пожаловать")
		for _, hint := range []string{"@" + botUsername + " np", "@" + botUsername + " recent", "/link"} {
			assert.Contains(t, text, hint)
		}
		assert.NotContains(t, text, "/invite")
	})

	t.Run("welcome explains who sees the user's music", func(t *testing.T) {
		s := newScenario(t)

		s.send(s.textMessage(alice, "/start"))

		text := lastReply(t, s).Text
		assert.Contains(t, text, "другие пользователи её не видят")
		assert.Contains(t, text, "владелец сервера")
	})

	t.Run("stranger learns what the service is", func(t *testing.T) {
		s := newScenario(t, withAdminContact("@boss_support"))
		s.share(alice, s.uploaded(alice, s.uploadAudio("track.mp3")), "🔗 Трек")
		s.uploaded(bob, s.uploadAudioFile(makeAudio(t, "song.mp3", audioSpec{Tags: songTags})))

		s.send(s.textMessage(stranger, "/start"))

		text := lastReply(t, s).Text
		assert.Contains(t, text, "Navidrome")
		assert.Contains(t, text, "Пользователей: 3")
		assert.Contains(t, text, "Треков в общей библиотеке: 1")
		assert.Contains(t, text, "@boss_support")
		assert.NotContains(t, text, "/link")
	})

	t.Run("stranger sees no contact unless it is set", func(t *testing.T) {
		s := newScenario(t)

		s.send(s.textMessage(stranger, "/start"))

		text := lastReply(t, s).Text
		assert.Contains(t, text, "Пользователей: 3")
		assert.NotContains(t, text, "Попросить доступ")
	})

	t.Run("admin also learns about invites", func(t *testing.T) {
		s := newScenario(t)

		s.send(s.textMessage(adminUser, "/start"))

		assert.Contains(t, lastReply(t, s).Text, "/invite")
	})
}
