package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/audiofile"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/telegram"
)

func TestHome(t *testing.T) {
	t.Parallel()

	t.Run("user sees home in Russian", func(t *testing.T) {
		s := harness.New(t)

		s.Open(alice)

		text := s.WindowText()
		assert.Contains(t, text, "Привет")
		for _, hint := range []string{"np", "recent", "shared", "top"} {
			assert.Contains(t, text, "@"+telegram.BotUsername+" "+hint)
		}
		assert.Contains(t, text, "https://github.com/lubaskinc0de/navidrome-tg")
		assert.Contains(t, text, "@lubaskinc0de")
	})

	t.Run("newcomer without language code sees home in English", func(t *testing.T) {
		s := harness.New(t)
		carol := harness.Newcomer("carol")
		carol.LanguageCode = ""
		s.Send(s.TextMessage(carol, "/start "+s.Invite()))

		s.Open(carol)

		assert.Contains(t, s.WindowText(), "Hi")
		assert.Contains(t, telegram.ButtonTexts(s.Telegram.Buttons(t)), "🌐 Русский")
	})

	t.Run("language switch redraws the window and survives start", func(t *testing.T) {
		s := harness.New(t)
		s.Open(alice)

		s.Go(alice, "🌐 English")
		switched := s.Telegram.Window()
		s.Open(alice)

		assert.Equal(t, "editMessageText", switched.Method)
		assert.Contains(t, switched.Params["text"], "Hi")
		assert.Contains(t, s.WindowText(), "Hi")
	})

	t.Run("second start removes the old window's buttons", func(t *testing.T) {
		s := harness.New(t)
		s.Open(alice)
		old := s.Telegram.Window().MessageID
		s.Telegram.Forget()

		s.Open(alice)

		assert.Equal(t, []int{old}, s.Telegram.StrippedMessages())
		assert.NotEqual(t, old, s.Telegram.Window().MessageID)
	})

	t.Run("only admin has the invite button", func(t *testing.T) {
		s := harness.New(t)

		s.Open(admin)
		adminButtons := telegram.ButtonTexts(s.Telegram.Buttons(t))
		s.Open(alice)

		assert.Contains(t, adminButtons, "🎟 Пригласить")
		assert.NotContains(t, telegram.ButtonTexts(s.Telegram.Buttons(t)), "🎟 Пригласить")
	})
}

func TestWindowOnInstances(t *testing.T) {
	t.Parallel()

	t.Run("starts on two instances leave one window with buttons", func(t *testing.T) {
		s := harness.New(t, harness.WithLeaseTTL(time.Minute))
		replica := s.StartReplica()
		first := s.Telegram.Hold("sendMessage")
		var starts sync.WaitGroup
		starts.Go(func() { s.Send(s.TextMessage(alice, "/start")) })
		<-first.Arrived()

		starts.Go(func() { replica.Send(replica.TextMessage(alice, "/start")) })
		assert.False(t, s.Telegram.WaitCalls("sendMessage", 1, anotherRound), "another instance acted meanwhile")
		first.Release()
		starts.Wait()

		assert.Len(t, s.Telegram.MessagesWithButtons(), 1)
	})

	t.Run("window works again after an instance crashed amid a redraw", func(t *testing.T) {
		s := harness.New(t)
		replica := s.StartReplica()
		redraw := s.Telegram.Hold("sendMessage")
		var start sync.WaitGroup
		start.Go(func() { replica.Send(replica.TextMessage(alice, "/start")) })
		<-redraw.Arrived()
		replica.Stop()
		start.Wait()

		s.Open(alice)

		assert.Contains(t, s.WindowText(), "Привет")
		assert.Len(t, s.Telegram.MessagesWithButtons(), 1)
	})

	t.Run("Imports refresh leaves the screen opened meanwhile on another instance", func(t *testing.T) {
		s := harness.New(t, harness.WithLeaseTTL(time.Minute))
		replica := s.StartReplica()
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.Zvuk.HoldStreams(0)
		s.ImportZvuk(alice)
		back := s.Button("← Назад")
		refresh := s.Telegram.Hold("editMessageText")
		edits := len(s.Telegram.CallsTo("editMessageText"))
		s.Zvuk.ReleaseStreams()
		<-refresh.Arrived()

		var press sync.WaitGroup
		press.Go(func() { replica.Press(alice, back) })
		assert.False(t, s.Telegram.WaitCalls("editMessageText", edits+1, anotherRound), "another instance acted meanwhile")
		refresh.Release()
		press.Wait()
		s.WaitIngest()

		assert.Contains(t, s.WindowText(), "Подключите сервис")
	})
}

func TestStrangerHome(t *testing.T) {
	t.Parallel()

	t.Run("stranger learns what the service is", func(t *testing.T) {
		s := harness.New(t, harness.WithAdminContact("@boss_support"))
		s.Share(alice, s.Uploaded(alice, s.UploadAudio("track.mp3")), "🔗 Трек")

		s.Open(stranger)

		text := s.WindowText()
		assert.Contains(t, text, "Navidrome")
		assert.Contains(t, text, "Пользователей: 3")
		assert.Contains(t, text, "Треков в общей библиотеке: 1")
		assert.Contains(t, text, "@boss_support")
		assert.Equal(t, []string{"🌐 English"}, telegram.ButtonTexts(s.Telegram.Buttons(t)))
	})

	t.Run("stranger sees no contact unless it is set", func(t *testing.T) {
		s := harness.New(t)

		s.Open(stranger)

		assert.NotContains(t, s.WindowText(), "Попросить доступ")
	})

	t.Run("stranger without language code sees English", func(t *testing.T) {
		s := harness.New(t)
		foreigner := harness.User{ID: 7777, Username: "foreigner"}

		s.Open(foreigner)

		assert.Contains(t, s.WindowText(), "invite-only")
	})

	t.Run("stranger switches the language", func(t *testing.T) {
		s := harness.New(t)
		s.Open(stranger)

		s.Go(stranger, "🌐 English")

		assert.Contains(t, s.WindowText(), "invite-only")
		assert.Equal(t, []string{"🌐 Русский"}, telegram.ButtonTexts(s.Telegram.Buttons(t)))
	})
}

func TestHowToUpload(t *testing.T) {
	t.Parallel()

	t.Run("how to upload explains formats, reactions and share", func(t *testing.T) {
		s := harness.New(t)

		s.Open(alice, "❓ Как загружать")

		text := s.WindowText()
		for _, hint := range []string{"mp3, flac", "👀", "👍", "👎", "/share", "владелец сервера"} {
			assert.Contains(t, text, hint)
		}
	})
}

func TestLanguage(t *testing.T) {
	t.Parallel()

	t.Run("upload of an unsupported format is refused in the user's language", func(t *testing.T) {
		s := harness.New(t)
		s.Open(alice, "🌐 English")
		upload := s.DocumentMessage(alice, s.UploadDocument(audiofile.Fixture("track.mp3"), "text/plain"))
		upload.Message.Document.FileName = "notes.txt"

		s.Send(upload)

		assert.Contains(t, s.LastReply().Text, "the format is not supported")
	})

	t.Run("share answers in the user's language", func(t *testing.T) {
		s := harness.New(t)
		upload := s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.Open(alice, "🌐 English")

		s.Send(s.ReplyCommand(alice, "/share", upload))

		assert.Equal(t, "What to Share?", s.LastReply().Text)
		assert.Equal(t, []string{"🔗 Track", "💿 Whole album"}, telegram.ButtonTexts(s.Telegram.Buttons(t)))
	})

	t.Run("inline answers follow the language switch", func(t *testing.T) {
		s := harness.New(t)
		s.Open(alice, "🌐 English")
		query := s.InlineQuery(alice, "top")

		s.Send(query)

		assert.Equal(t, "🏆 Top", s.Telegram.InlineAnswerTo(t, query).Results[0].Title)
	})
}

func TestBotDescription(t *testing.T) {
	t.Parallel()

	t.Run("description is set in English by default and in Russian", func(t *testing.T) {
		s := harness.New(t)

		s.Restart()

		descriptions := map[string]string{}
		for _, call := range s.Telegram.CallsTo("setMyDescription") {
			descriptions[call.Params["language_code"]] = call.Params["description"]
		}
		assert.Contains(t, descriptions[""], "your music library")
		assert.Contains(t, descriptions["ru"], "вашу музыкальную библиотеку")
		assert.Len(t, s.Telegram.CallsTo("setMyShortDescription"), 2)
	})

	t.Run("command menu has only start", func(t *testing.T) {
		s := harness.New(t)

		s.Restart()

		commands := s.Telegram.CallsTo("setMyCommands")
		require.Len(t, commands, 2)
		for _, call := range commands {
			var menu []struct {
				Command string `json:"command"`
			}
			require.NoError(t, json.Unmarshal([]byte(call.Params["commands"]), &menu))
			require.Len(t, menu, 1)
			assert.Equal(t, "start", menu[0].Command)
		}
	})
}

func TestListen(t *testing.T) {
	t.Parallel()

	t.Run("how to listen gives the Navidrome address and players", func(t *testing.T) {
		s := harness.New(t)

		s.Open(alice, "📱 Как слушать")

		text := s.WindowText()
		for _, hint := range []string{harness.NavidromePublicURL, "Symfonium", "Amperfy", "Subsonic"} {
			assert.Contains(t, text, hint)
		}
	})

	t.Run("registration message gives the Navidrome address", func(t *testing.T) {
		s := harness.New(t)

		s.Register(harness.Newcomer("carol"))

		assert.Contains(t, s.LastReply().Text, harness.NavidromePublicURL)
	})
}

func TestTranslations(t *testing.T) {
	t.Parallel()

	t.Run("service name comes from the config", func(t *testing.T) {
		s := harness.New(t, harness.WithServiceName("Музыкалка"))

		s.Open(alice)

		assert.Contains(t, s.WindowText(), "Музыкалка переносит")
	})

	t.Run("added language is offered in the language list", func(t *testing.T) {
		s := harness.New(t, harness.WithTranslations(translations(t, map[string]string{
			"de.yaml": "language:\n  name: Deutsch\nhome:\n  greeting_named: 👋 Hallo, {{.Name}}!\n",
		})))
		s.Open(alice, "🌐 Язык")
		languages := telegram.ButtonTexts(s.Telegram.Buttons(t))

		s.Go(alice, "🌐 Deutsch")

		assert.Equal(t, []string{"🌐 Deutsch", "🌐 English", "🌐 Русский", "← Назад"}, languages)
		assert.Contains(t, s.WindowText(), "👋 Hallo, alice!")
		assert.Contains(t, telegram.ButtonTexts(s.Telegram.Buttons(t)), "🎵 Feed")
	})

	t.Run("client language picks the added language", func(t *testing.T) {
		s := harness.New(t, harness.WithTranslations(translations(t, map[string]string{
			"de.yaml": "language:\n  name: Deutsch\nstranger:\n  contact: 'Zugang: {{.Contact}}'\n",
		})), harness.WithAdminContact("@boss"))
		german := harness.User{ID: 7777, Username: "hans", LanguageCode: "de-AT"}

		s.Open(german)

		assert.Contains(t, s.WindowText(), "Zugang: @boss")
	})

	t.Run("file of a built-in language overrides its texts", func(t *testing.T) {
		s := harness.New(t, harness.WithTranslations(translations(t, map[string]string{
			"ru.yaml": "button:\n  feed: 🎶 Музыка друзей\n",
		})))

		s.Open(alice, "🎶 Музыка друзей")

		assert.Contains(t, s.WindowText(), "Пока никто ничем не поделился")
	})

	t.Run("source link and author stay whatever the translation says", func(t *testing.T) {
		s := harness.New(t, harness.WithTranslations(translations(t, map[string]string{
			"ru.yaml": "home:\n  text: Просто бот\nfooter:\n  source: ''\n  author: ''\n",
		})))

		s.Open(alice)

		assert.Contains(t, s.WindowText(), "https://github.com/lubaskinc0de/navidrome-tg")
		assert.Contains(t, s.WindowText(), "@lubaskinc0de")
	})
}

func translations(t *testing.T, files map[string]string) string {
	t.Helper()

	dir := t.TempDir()
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
	}
	return dir
}
