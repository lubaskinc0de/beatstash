package e2e

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/beatstash/e2e/harness"
	"github.com/lubaskinc0de/beatstash/e2e/harness/audiofile"
	"github.com/lubaskinc0de/beatstash/e2e/harness/telegram"
	"github.com/lubaskinc0de/beatstash/internal/application/share_tracks"
	"github.com/lubaskinc0de/beatstash/internal/application/view_top"
	"github.com/lubaskinc0de/beatstash/internal/domain/ingest"
	"github.com/lubaskinc0de/beatstash/internal/domain/library"
)

func TestHome(t *testing.T) {
	t.Parallel()

	t.Run("user sees home in Russian", func(t *testing.T) {
		s := harness.New(t)

		s.Open(alice)

		text := s.WindowText()
		assert.Contains(t, text, s.Catalog(alice).Home(alice.Username, telegram.BotUsername, library.Usage{}))
		assert.Contains(t, text, "https://github.com/lubaskinc0de/beatstash")
		assert.Contains(t, text, "@lubaskinc0de")
	})

	t.Run("newcomer without language code sees home in English", func(t *testing.T) {
		s := harness.New(t)
		carol := harness.Newcomer("carol")
		carol.LanguageCode = ""
		s.Send(s.TextMessage(carol, "/start "+s.Invite()))

		s.Open(carol)

		assert.Contains(t, s.WindowText(), s.Catalog(carol).Home(carol.Username, telegram.BotUsername, library.Usage{}))
		s.Go(carol, s.Catalog(carol).SettingsButton())
		assert.Contains(t, telegram.ButtonTexts(s.Telegram.Buttons(t)), s.Catalog(alice).LanguageButton())
	})

	t.Run("second start deletes the old window", func(t *testing.T) {
		s := harness.New(t)
		s.Open(alice)
		old := s.Telegram.Window().MessageID
		s.Telegram.Forget()

		s.Open(alice)

		assert.Contains(t, s.Telegram.DeletedMessages(), strconv.Itoa(old))
		assert.Empty(t, s.Telegram.StrippedMessages())
		assert.NotEqual(t, old, s.Telegram.Window().MessageID)
	})

	t.Run("old window Telegram keeps loses its buttons", func(t *testing.T) {
		s := harness.New(t)
		s.Open(alice)
		old := s.Telegram.Window().MessageID
		s.Telegram.FailCalls("deleteMessage", 1, http.StatusBadRequest)

		s.Open(alice)

		assert.Equal(t, []int{old}, s.Telegram.StrippedMessages())
	})

	t.Run("failed new window leaves the old one", func(t *testing.T) {
		s := harness.New(t)
		s.Open(alice)
		old := s.Telegram.Window().MessageID
		s.Telegram.FailCalls("sendMessage", 1, http.StatusInternalServerError)

		s.Open(alice)

		assert.NotContains(t, s.Telegram.DeletedMessages(), strconv.Itoa(old))
		assert.Empty(t, s.Telegram.StrippedMessages())
	})

	t.Run("start command is deleted", func(t *testing.T) {
		s := harness.New(t)
		start := s.TextMessage(alice, "/start")

		s.Send(start)

		assert.Contains(t, s.Telegram.DeletedMessages(), strconv.Itoa(start.Message.ID))
	})

	t.Run("menu has music, import, help and settings", func(t *testing.T) {
		s := harness.New(t)

		s.Open(alice)

		c := s.Catalog(alice)
		assert.Equal(t, []string{c.MusicButton(), c.ImportButton(), c.HelpButton(), c.SettingsButton()}, telegram.ButtonTexts(s.Telegram.Buttons(t)))
	})

	t.Run("admin also has the admin screen with invites", func(t *testing.T) {
		s := harness.New(t)

		s.Open(admin)
		menu := telegram.ButtonTexts(s.Telegram.Buttons(t))
		s.Go(admin, s.Catalog(admin).AdminButton())

		c := s.Catalog(admin)
		assert.Equal(t, []string{c.MusicButton(), c.ImportButton(), c.HelpButton(), c.SettingsButton(), c.AdminButton()}, menu)
		assert.Contains(t, telegram.ButtonTexts(s.Telegram.Buttons(t)), c.InviteButton())
	})
}

// replicaResponseWindow is how long a replica blocked on a window lease gets
// to show it is not blocked: nothing tells that it tried and waits.
const replicaResponseWindow = 500 * time.Millisecond

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
		assert.False(t, s.Telegram.WaitCalls("sendMessage", 1, replicaResponseWindow), "another instance acted meanwhile")
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

		assert.Contains(t, s.WindowText(), s.Catalog(alice).Home(alice.Username, telegram.BotUsername, library.Usage{}))
		assert.Len(t, s.Telegram.MessagesWithButtons(), 1)
	})

	t.Run("Imports refresh leaves the screen opened meanwhile on another instance", func(t *testing.T) {
		s := harness.New(t, harness.WithLeaseTTL(time.Minute))
		replica := s.StartReplica()
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.Zvuk.HoldStreams(0)
		s.ImportZvuk(alice)
		back := s.Button(s.Catalog(alice).Back())
		refresh := s.Telegram.Hold("editMessageText")
		edits := len(s.Telegram.CallsTo("editMessageText"))
		s.Zvuk.ReleaseStreams()
		<-refresh.Arrived()

		var press sync.WaitGroup
		press.Go(func() { replica.Press(alice, back) })
		assert.False(t, s.Telegram.WaitCalls("editMessageText", edits+1, replicaResponseWindow), "another instance acted meanwhile")
		refresh.Release()
		press.Wait()
		s.WaitIngest()

		assert.Contains(t, s.WindowText(), s.Catalog(alice).ImportSources())
	})
}

func TestStrangerHome(t *testing.T) {
	t.Parallel()

	t.Run("stranger learns what the service is", func(t *testing.T) {
		s := harness.New(t, harness.WithAdminContact("@boss_support"))
		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.ShareTrack(alice, fixtureButton)

		s.Open(stranger)

		text := s.WindowText()
		assert.Contains(t, text, "Navidrome")
		assert.Contains(t, text, s.Catalog(stranger).StrangerHome(3, 1, "@boss_support"))
		assert.Contains(t, text, "@boss_support")
		assert.Equal(t, []string{s.Catalog(harness.User{LanguageCode: "en"}).LanguageButton()}, telegram.ButtonTexts(s.Telegram.Buttons(t)))
	})

	t.Run("stranger sees no contact unless it is set", func(t *testing.T) {
		s := harness.New(t)

		s.Open(stranger)

		assert.Equal(t, s.Catalog(stranger).StrangerHome(3, 0, ""), s.WindowText())
	})

	t.Run("stranger without language code sees English", func(t *testing.T) {
		s := harness.New(t)
		foreigner := harness.User{ID: 7777, Username: "foreigner"}

		s.Open(foreigner)

		assert.Contains(t, s.WindowText(), s.Catalog(foreigner).StrangerHome(3, 0, ""))
	})

	t.Run("stranger switches the language", func(t *testing.T) {
		s := harness.New(t)
		s.Open(stranger)

		s.Go(stranger, s.Catalog(harness.User{LanguageCode: "en"}).LanguageButton())

		english := stranger
		english.LanguageCode = "en"
		assert.Contains(t, s.WindowText(), s.Catalog(english).StrangerHome(3, 0, ""))
		assert.Equal(t, []string{s.Catalog(stranger).LanguageButton()}, telegram.ButtonTexts(s.Telegram.Buttons(t)))
	})
}

func TestHelp(t *testing.T) {
	t.Parallel()

	t.Run("help has four sections", func(t *testing.T) {
		s := harness.New(t)

		s.Open(alice, s.Catalog(alice).HelpButton())

		c := s.Catalog(alice)
		assert.Equal(t, []string{c.HowToButton(), c.ListenButton(), c.InlineHelpButton(), c.SpaceHelpButton(), c.Back()}, telegram.ButtonTexts(s.Telegram.Buttons(t)))
	})

	t.Run("section leads back to help", func(t *testing.T) {
		s := harness.New(t)

		s.Open(alice, s.Catalog(alice).HelpButton(), s.Catalog(alice).ListenButton(), s.Catalog(alice).Back())

		assert.Contains(t, telegram.ButtonTexts(s.Telegram.Buttons(t)), s.Catalog(alice).SpaceHelpButton())
	})

	t.Run("other chats section lists the inline commands", func(t *testing.T) {
		s := harness.New(t)

		s.Open(alice, s.Catalog(alice).HelpButton(), s.Catalog(alice).InlineHelpButton())

		for _, hint := range []string{"np", "recent", "shared", "top"} {
			assert.Contains(t, s.WindowText(), "@"+telegram.BotUsername+" "+hint)
		}
	})

	t.Run("space section tells whom to ask for more", func(t *testing.T) {
		s := harness.New(t, harness.WithAdminContact("@boss_support"))

		s.Open(alice, s.Catalog(alice).HelpButton(), s.Catalog(alice).SpaceHelpButton())

		assert.Contains(t, s.WindowText(), s.Catalog(alice).SpaceHelp("@boss_support"))
	})

	t.Run("how to upload explains formats, reactions and share", func(t *testing.T) {
		s := harness.New(t)

		s.Open(alice, s.Catalog(alice).HelpButton(), s.Catalog(alice).HowToButton())

		text := s.WindowText()
		assert.Contains(t, text, s.Catalog(alice).HowTo())
		for _, marker := range []string{"mp3", "flac", "👀", "👍", "👎"} {
			assert.Contains(t, text, marker)
		}
	})
}

func TestSettings(t *testing.T) {
	t.Parallel()

	t.Run("settings have the account and the language", func(t *testing.T) {
		s := harness.New(t)

		s.Open(alice, s.Catalog(alice).SettingsButton())

		c := s.Catalog(alice)
		english := s.Catalog(harness.User{LanguageCode: "en"})
		assert.Equal(t, []string{c.AccountsButton(), english.LanguageButton(), c.Back()}, telegram.ButtonTexts(s.Telegram.Buttons(t)))
	})

	t.Run("account screen leads back to settings", func(t *testing.T) {
		s := harness.New(t)

		s.Open(alice, s.Catalog(alice).SettingsButton(), s.Catalog(alice).AccountsButton(), s.Catalog(alice).Back())

		assert.Contains(t, s.WindowText(), s.Catalog(alice).Settings())
	})

	t.Run("language switch redraws the settings and survives start", func(t *testing.T) {
		s := harness.New(t)
		s.Open(alice, s.Catalog(alice).SettingsButton())

		s.Go(alice, s.Catalog(harness.User{LanguageCode: "en"}).LanguageButton())
		switched := s.Telegram.Window()
		s.Open(alice)
		english := alice
		english.LanguageCode = "en"

		assert.Equal(t, "editMessageText", switched.Method)
		assert.Contains(t, switched.Params["text"], s.Catalog(english).Settings())
		assert.Contains(t, s.WindowText(), s.Catalog(english).Home(alice.Username, telegram.BotUsername, library.Usage{}))
	})
}

func TestLanguage(t *testing.T) {
	t.Parallel()

	t.Run("upload of an unsupported format is refused in the user's language", func(t *testing.T) {
		s := harness.New(t)
		s.Open(alice, s.Catalog(alice).SettingsButton(), s.Catalog(harness.User{LanguageCode: "en"}).LanguageButton())
		upload := s.DocumentMessage(alice, s.UploadDocument(audiofile.Fixture("track.mp3"), "text/plain"))
		upload.Message.Document.FileName = "notes.txt"

		s.Send(upload)

		english := alice
		english.LanguageCode = "en"
		assert.Contains(t, s.LastReply().Text, s.Catalog(english).UploadFailed(ingest.ReasonUnsupportedFormat))
	})

	t.Run("share answers in the user's language", func(t *testing.T) {
		s := harness.New(t)
		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.Open(alice, s.Catalog(alice).SettingsButton(), s.Catalog(harness.User{LanguageCode: "en"}).LanguageButton())
		english := alice
		english.LanguageCode = "en"

		s.ShareTrack(english, fixtureButton)

		assert.Equal(t, s.Catalog(english).ShareResult(&share_tracks.ShareResult{Created: 1}), s.LastCallbackAnswer())
	})

	t.Run("inline answers follow the language switch", func(t *testing.T) {
		s := harness.New(t)
		s.Open(alice, s.Catalog(alice).SettingsButton(), s.Catalog(harness.User{LanguageCode: "en"}).LanguageButton())
		query := s.InlineQuery(alice, "top")

		s.Send(query)

		english := alice
		english.LanguageCode = "en"
		assert.Equal(t, s.Catalog(english).TopArticle(&view_top.Top{}).Title, s.Telegram.InlineAnswerTo(t, query).Results[0].Title)
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
		assert.Contains(t, descriptions[""], s.Catalog(harness.User{LanguageCode: "en"}).BotDescription())
		assert.Contains(t, descriptions["ru"], s.Catalog(alice).BotDescription())
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

		s.Open(alice, s.Catalog(alice).HelpButton(), s.Catalog(alice).ListenButton())

		text := s.WindowText()
		assert.Contains(t, text, s.Catalog(alice).Listen())
		for _, marker := range []string{harness.NavidromePublicURL, "Symfonium", "Amperfy", "Subsonic"} {
			assert.Contains(t, text, marker)
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

		assert.Contains(t, s.WindowText(), s.Catalog(alice).Home(alice.Username, telegram.BotUsername, library.Usage{}))
		assert.Contains(t, s.WindowText(), "Музыкалка")
	})

	t.Run("added language is offered in the language list", func(t *testing.T) {
		s := harness.New(t, harness.WithTranslations(translations(t, map[string]string{
			"de.yaml": "language:\n  name: Deutsch\nhome:\n  greeting_named: 👋 Hallo, {{.Name}}!\n",
		})))
		s.Open(alice, s.Catalog(alice).SettingsButton(), s.Catalog(alice).LanguagesButton())
		languages := telegram.ButtonTexts(s.Telegram.Buttons(t))

		s.Go(alice, s.Catalog(harness.User{LanguageCode: "de"}).LanguageButton())

		c := s.Catalog(alice)
		assert.Equal(t, []string{
			s.Catalog(harness.User{LanguageCode: "de"}).LanguageButton(),
			s.Catalog(harness.User{LanguageCode: "en"}).LanguageButton(),
			c.LanguageButton(), c.Back(),
		}, languages)
		german := alice
		german.LanguageCode = "de"
		s.Open(alice)
		assert.Contains(t, s.WindowText(), s.Catalog(german).Home(alice.Username, telegram.BotUsername, library.Usage{}))
		assert.Contains(t, s.WindowText(), "Hallo, alice!")
		assert.Contains(t, telegram.ButtonTexts(s.Telegram.Buttons(t)), s.Catalog(german).MusicButton())
	})

	t.Run("client language picks the added language", func(t *testing.T) {
		s := harness.New(t, harness.WithTranslations(translations(t, map[string]string{
			"de.yaml": "language:\n  name: Deutsch\nstranger:\n  contact: 'Zugang: {{.Contact}}'\n",
		})), harness.WithAdminContact("@boss"))
		german := harness.User{ID: 7777, Username: "hans", LanguageCode: "de-AT"}

		s.Open(german)

		assert.Contains(t, s.WindowText(), s.Catalog(german).StrangerHome(3, 0, "@boss"))
		assert.Contains(t, s.WindowText(), "Zugang: @boss")
	})

	t.Run("file of a built-in language overrides its texts", func(t *testing.T) {
		s := harness.New(t, harness.WithTranslations(translations(t, map[string]string{
			"ru.yaml": "button:\n  music: 🎶 Музыка друзей\n",
		})))

		s.Open(alice)
		assert.Contains(t, telegram.ButtonTexts(s.Telegram.Buttons(t)), "🎶 Музыка друзей")
		s.Go(alice, s.Catalog(alice).MusicButton())

		assert.Contains(t, s.WindowText(), s.Catalog(alice).FeedEmpty())
	})

	t.Run("source link and author stay whatever the translation says", func(t *testing.T) {
		s := harness.New(t, harness.WithTranslations(translations(t, map[string]string{
			"ru.yaml": "home:\n  text: Просто бот\nfooter:\n  source: ''\n  author: ''\n",
		})))

		s.Open(alice)

		assert.Contains(t, s.WindowText(), s.Catalog(alice).Home(alice.Username, telegram.BotUsername, library.Usage{}))
		assert.Contains(t, s.WindowText(), "https://github.com/lubaskinc0de/beatstash")
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
