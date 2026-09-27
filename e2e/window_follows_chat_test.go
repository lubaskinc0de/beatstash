package e2e

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
)

func TestWindowFollowsChat(t *testing.T) {
	t.Parallel()

	t.Run("press with nothing below edits the window", func(t *testing.T) {
		s := harness.New(t)
		s.Open(alice)
		old := s.Telegram.Window().MessageID

		s.Go(alice, "🎵 Лента")

		window := s.Telegram.Window()
		assert.Equal(t, "editMessageText", window.Method)
		assert.Equal(t, old, window.MessageID)
		assert.Contains(t, s.WindowText(), "Пока никто ничем не поделился")
	})

	t.Run("press after an upload sends the next screen below", func(t *testing.T) {
		s := harness.New(t)
		s.Open(alice)
		old := s.Telegram.Window().MessageID
		s.Uploaded(alice, s.UploadAudio("track.mp3"))

		s.Go(alice, "🎵 Лента")

		window := s.Telegram.Window()
		assert.Equal(t, "sendMessage", window.Method)
		assert.Greater(t, window.MessageID, old)
		assert.Contains(t, s.WindowText(), "Пока никто ничем не поделился")
		assert.Contains(t, s.Telegram.StrippedMessages(), old)
		assert.NotContains(t, s.Telegram.EditedMessages(), strconv.Itoa(old))
	})

	t.Run("window moves below after a restart", func(t *testing.T) {
		s := harness.New(t)
		s.Open(alice)
		old := s.Telegram.Window().MessageID
		feed := s.Button("🎵 Лента")
		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.Restart()

		s.PressOn(alice, feed, old)

		assert.Equal(t, "sendMessage", s.Telegram.Window().Method)
		assert.Contains(t, s.Telegram.StrippedMessages(), old)
	})

	t.Run("moved window shows the result of the press", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.OpenZvuk(alice)
		s.Uploaded(alice, s.UploadAudio("track.mp3"))

		s.Go(alice, "🚫 Отключить")

		assert.Equal(t, "sendMessage", s.Telegram.Window().Method)
		assert.Contains(t, s.WindowText(), "Звук отключён")
		assert.Contains(t, s.WindowText(), "Статус: не подключён")
	})

	t.Run("moved window keeps the screen's argument", func(t *testing.T) {
		s := harness.New(t)
		s.Zvuk.AddAccount(harness.ZvukToken, true)
		s.OpenZvuk(alice)
		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.Go(alice, "🔌 Подключить")

		s.SendText(alice, harness.ZvukToken)

		assert.Contains(t, s.WindowText(), "Звук подключён")
	})

	t.Run("language list moves below", func(t *testing.T) {
		s := harness.New(t, harness.WithTranslations(translations(t, map[string]string{
			"de.yaml": "language:\n  name: Deutsch\n",
		})))
		s.Open(alice)
		old := s.Telegram.Window().MessageID
		s.Uploaded(alice, s.UploadAudio("track.mp3"))

		s.Go(alice, "🌐 Язык")

		assert.Equal(t, "sendMessage", s.Telegram.Window().Method)
		assert.Contains(t, s.Telegram.StrippedMessages(), old)
		assert.Contains(t, s.WindowText(), "Выберите язык")
	})

	t.Run("language switch works in the moved list", func(t *testing.T) {
		s := harness.New(t, harness.WithTranslations(translations(t, map[string]string{
			"de.yaml": "language:\n  name: Deutsch\n",
		})))
		s.Open(alice)
		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.Go(alice, "🌐 Язык")
		moved := s.Telegram.Window().MessageID

		s.Go(alice, "🌐 English")

		assert.Equal(t, moved, s.Telegram.Window().MessageID)
		assert.Contains(t, s.WindowText(), "Hi")
	})

	t.Run("press on an old window opens a new one below", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.OpenZvuk(alice)
		old := s.Telegram.Window().MessageID
		disconnect := s.Button("🚫 Отключить")
		s.Open(alice)
		s.Restart()
		s.Telegram.Forget()

		s.PressOn(alice, disconnect, old)

		assert.Equal(t, "sendMessage", s.Telegram.Window().Method)
		assert.Contains(t, s.WindowText(), "Звук отключён")
		assert.Empty(t, s.Telegram.EditedMessages())
	})

	t.Run("text answer shows the result below it", func(t *testing.T) {
		s := harness.New(t)
		carol := harness.Newcomer("carol")
		s.Send(s.TextMessage(carol, "/start "+s.Invite()))
		old := s.Telegram.Window().MessageID

		s.SendText(carol, "carol")

		window := s.Telegram.Window()
		assert.Equal(t, "sendMessage", window.Method)
		assert.Greater(t, window.MessageID, old)
		assert.Contains(t, s.Telegram.StrippedMessages(), old)
	})

	t.Run("deleted secret leaves the window in place", func(t *testing.T) {
		s := harness.New(t)
		s.Zvuk.AddAccount(harness.ZvukToken, true)
		s.OpenZvuk(alice, "🔌 Подключить")
		old := s.Telegram.Window().MessageID

		s.SendText(alice, harness.ZvukToken)

		window := s.Telegram.Window()
		assert.Equal(t, "editMessageText", window.Method)
		assert.Equal(t, old, window.MessageID)
		assert.Contains(t, s.WindowText(), "Звук подключён")
	})

	t.Run("uploads alone move no window", func(t *testing.T) {
		s := harness.New(t)
		s.Open(alice)
		old := s.Telegram.Window().MessageID

		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.Uploaded(alice, s.UploadAudio("track.flac"))

		assert.Equal(t, old, s.Telegram.Window().MessageID)
		assert.NotContains(t, s.Telegram.StrippedMessages(), old)
	})

	t.Run("Poller redraws the Imports in place under an upload and a summary", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.Zvuk.HoldStreams(0)
		s.ImportZvuk(alice)
		imports := s.Telegram.Window().MessageID
		s.Send(s.AudioMessage(alice, s.UploadAudio("track.mp3")))

		s.Zvuk.ReleaseStreams()
		s.WaitIngest()

		window := s.Telegram.Window()
		assert.Equal(t, "editMessageText", window.Method)
		assert.Equal(t, imports, window.MessageID)
		assert.Contains(t, s.WindowText(), "Сейчас ничего не импортируется")
	})
}
