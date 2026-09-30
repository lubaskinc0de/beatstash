package e2e

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/telegram"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

func TestWindowFollowsChat(t *testing.T) {
	t.Parallel()

	t.Run("press with nothing below edits the window", func(t *testing.T) {
		s := harness.New(t)
		s.Open(alice)
		old := s.Telegram.Window().MessageID

		s.Go(alice, s.Catalog(alice).FeedButton())

		c := s.Catalog(alice)
		window := s.Telegram.Window()
		assert.Equal(t, "editMessageText", window.Method)
		assert.Equal(t, old, window.MessageID)
		assert.Contains(t, s.WindowText(), c.FeedEmpty())
	})

	t.Run("press after an upload sends the next screen below", func(t *testing.T) {
		s := harness.New(t)
		s.Open(alice)
		old := s.Telegram.Window().MessageID
		s.Uploaded(alice, s.UploadAudio("track.mp3"))

		s.Go(alice, s.Catalog(alice).FeedButton())

		window := s.Telegram.Window()
		assert.Equal(t, "sendMessage", window.Method)
		assert.Greater(t, window.MessageID, old)
		assert.Contains(t, s.WindowText(), s.Catalog(alice).FeedEmpty())
		assert.Contains(t, s.Telegram.StrippedMessages(), old)
		assert.NotContains(t, s.Telegram.EditedMessages(), strconv.Itoa(old))
	})

	t.Run("window moves below after a restart", func(t *testing.T) {
		s := harness.New(t)
		s.Open(alice)
		old := s.Telegram.Window().MessageID
		feed := s.Button(s.Catalog(alice).FeedButton())
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

		s.Go(alice, s.Catalog(alice).Disconnect())

		assert.Equal(t, "sendMessage", s.Telegram.Window().Method)
		assert.Contains(t, s.WindowText(), s.Catalog(alice).Disconnected("zvuk"))
		assert.Contains(t, s.WindowText(), s.Catalog(alice).Provider("zvuk", "not_connected"))
	})

	t.Run("moved window keeps the screen's argument", func(t *testing.T) {
		s := harness.New(t)
		s.Zvuk.AddAccount(harness.ZvukToken, true)
		s.OpenZvuk(alice)
		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.Go(alice, s.Catalog(alice).Connect())

		s.SendText(alice, harness.ZvukToken)

		assert.Contains(t, s.WindowText(), s.Catalog(alice).Connected("zvuk"))
	})

	t.Run("language list moves below and still switches language", func(t *testing.T) {
		s := harness.New(t, harness.WithTranslations(translations(t, map[string]string{
			"de.yaml": "language:\n  name: Deutsch\n",
		})))
		s.Open(alice)
		old := s.Telegram.Window().MessageID
		s.Uploaded(alice, s.UploadAudio("track.mp3"))

		s.Go(alice, s.Catalog(alice).LanguagesButton())

		moved := s.Telegram.Window().MessageID
		assert.Equal(t, "sendMessage", s.Telegram.Window().Method)
		assert.Contains(t, s.Telegram.StrippedMessages(), old)
		assert.Contains(t, s.WindowText(), s.Catalog(alice).ChooseLanguage())

		s.Go(alice, s.Catalog(harness.User{LanguageCode: "en"}).LanguageButton())

		assert.Equal(t, moved, s.Telegram.Window().MessageID)
		english := alice
		english.LanguageCode = "en"
		assert.Contains(t, s.WindowText(), s.Catalog(english).Home(alice.Username, telegram.BotUsername, library.Usage{}))
	})

	t.Run("press on an old window opens a new one below", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.OpenZvuk(alice)
		old := s.Telegram.Window().MessageID
		disconnect := s.Button(s.Catalog(alice).Disconnect())
		s.Open(alice)
		s.Restart()
		s.Telegram.Forget()

		s.PressOn(alice, disconnect, old)

		assert.Equal(t, "sendMessage", s.Telegram.Window().Method)
		assert.Contains(t, s.WindowText(), s.Catalog(alice).Disconnected("zvuk"))
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
		s.OpenZvuk(alice, s.Catalog(alice).Connect())
		old := s.Telegram.Window().MessageID

		s.SendText(alice, harness.ZvukToken)

		window := s.Telegram.Window()
		assert.Equal(t, "editMessageText", window.Method)
		assert.Equal(t, old, window.MessageID)
		assert.Contains(t, s.WindowText(), s.Catalog(alice).Connected("zvuk"))
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
		assert.Contains(t, s.WindowText(), s.Catalog(alice).Imports(nil))
	})
}
