package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/audiofile"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/telegram"
)

func TestFirstContact(t *testing.T) {
	t.Parallel()

	t.Run("new user starting with inline np gets an answer", func(t *testing.T) {
		s := harness.New(t)
		query := s.InlineQuery(alice, "np")

		s.Send(query)

		answers := s.Telegram.InlineAnswers(t)
		require.Len(t, answers, 1)
		assert.Equal(t, query.InlineQuery.ID, answers[0].QueryID)
	})

	t.Run("new user pressing a button gets an answer", func(t *testing.T) {
		s := harness.New(t)
		press := s.CallbackQuery(alice, "any")

		s.Send(press)

		assert.Equal(t, []string{press.CallbackQuery.ID}, s.Telegram.AnsweredCallbacks())
	})
}

func TestStart(t *testing.T) {
	t.Parallel()

	t.Run("user gets a welcome with instructions", func(t *testing.T) {
		s := harness.New(t)

		s.Send(s.TextMessage(alice, "/start"))

		text := s.LastReply().Text
		assert.Contains(t, text, "Добро пожаловать")
		for _, hint := range []string{"@" + telegram.BotUsername + " np", "@" + telegram.BotUsername + " recent", "/link", "/zvuk", "/zvuk_import", "zvuk.com/api/tiny/profile"} {
			assert.Contains(t, text, hint)
		}
		assert.NotContains(t, text, "/invite")
	})

	t.Run("welcome explains who sees the user's music", func(t *testing.T) {
		s := harness.New(t)

		s.Send(s.TextMessage(alice, "/start"))

		text := s.LastReply().Text
		assert.Contains(t, text, "другие пользователи её не видят")
		assert.Contains(t, text, "владелец сервера")
	})

	t.Run("stranger learns what the service is", func(t *testing.T) {
		s := harness.New(t, harness.WithAdminContact("@boss_support"))
		s.Share(alice, s.Uploaded(alice, s.UploadAudio("track.mp3")), "🔗 Трек")
		s.Uploaded(bob, s.UploadAudioFile(audiofile.Generate(t, "song.mp3", audiofile.Spec{Tags: audiofile.SongTags})))

		s.Send(s.TextMessage(stranger, "/start"))

		text := s.LastReply().Text
		assert.Contains(t, text, "Navidrome")
		assert.Contains(t, text, "Пользователей: 3")
		assert.Contains(t, text, "Треков в общей библиотеке: 1")
		assert.Contains(t, text, "@boss_support")
		assert.NotContains(t, text, "/link")
	})

	t.Run("stranger sees no contact unless it is set", func(t *testing.T) {
		s := harness.New(t)

		s.Send(s.TextMessage(stranger, "/start"))

		text := s.LastReply().Text
		assert.Contains(t, text, "Пользователей: 3")
		assert.NotContains(t, text, "Попросить доступ")
	})

	t.Run("admin also learns about invites", func(t *testing.T) {
		s := harness.New(t)

		s.Send(s.TextMessage(admin, "/start"))

		assert.Contains(t, s.LastReply().Text, "/invite")
	})
}
