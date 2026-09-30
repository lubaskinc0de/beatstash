package e2e

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/telegram"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/zvuk"
)

func TestZvukImport(t *testing.T) {
	t.Parallel()

	t.Run("Import brings each track of the collection once", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)

		s.ImportZvuk(alice)
		s.WaitIngest()

		files := s.PersonalFiles(alice)
		assert.Len(t, files, 9)
		assert.Contains(t, files, "Zvuk Band/Saved (2020)/01 - Song 1.mp3")
		assert.Contains(t, files, "Zvuk Band/Liked (2020)/05 - Song 5.mp3")
		assert.Contains(t, files, "Zvuk Band/Listed (2020)/01 - Song 1.mp3")
	})

	t.Run("bot tells the size before the Import starts", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)

		s.OpenZvuk(alice, "📥 Импортировать")

		text := s.WindowText()
		assert.Contains(t, text, "9 треков")
		assert.Contains(t, text, "МБ")
		assert.Empty(t, s.PersonalFiles(alice))
	})

	t.Run("repeated Import offers only the tracks the library lacks", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.ImportZvuk(alice)
		s.WaitIngest()
		fresh := s.AddZvukAlbum("675", "Fresh", 1)
		s.Zvuk.Update(harness.ZvukToken, func(a *zvuk.Account) { a.Liked = append(a.Liked, fresh...) })

		s.OpenZvuk(alice, "📥 Импортировать")

		assert.Contains(t, s.WindowText(), "В коллекции Звука 10 треков, в библиотеке ещё нет 1 трек")
	})

	t.Run("repeated Import of a stored collection offers nothing", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.ImportZvuk(alice)
		s.WaitIngest()

		s.OpenZvuk(alice, "📥 Импортировать")

		assert.Contains(t, s.WindowText(), "Вся коллекция Звука (9 треков) уже в вашей библиотеке")
		assert.Equal(t, []string{"← Назад"}, telegram.ButtonTexts(s.Telegram.Buttons(t)))
	})

	t.Run("button of an old offer tells the collection is stored", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.OpenZvuk(alice, "📥 Импортировать")
		start := s.Button("▶️ Начать")
		s.Press(alice, start)
		s.WaitIngest()

		s.Press(alice, start)

		assert.Contains(t, s.Telegram.CallbackAnswers(), "Вся коллекция уже в библиотеке")
	})

	t.Run("second tap on the button starts no second Import", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.Zvuk.HoldStreams(0)
		s.OpenZvuk(alice, "📥 Импортировать")
		start := s.Button("▶️ Начать")
		s.Press(alice, start)

		s.Press(alice, start)

		assert.Contains(t, s.Telegram.CallbackAnswers(), "Импорт уже идёт")
		assert.Equal(t, 1, strings.Count(s.WindowText(), "📥 <b>Звук</b>"))
	})

	t.Run("unconnected Zvuk offers only connecting", func(t *testing.T) {
		s := harness.New(t)

		s.OpenZvuk(alice)

		assert.Equal(t, []string{"🔌 Подключить", "← Назад"}, telegram.ButtonTexts(s.Telegram.Buttons(t)))
	})

	t.Run("interrupted Import finishes after restart without downloading twice", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		collection := s.AddZvukCollection(harness.ZvukToken)
		held := s.Zvuk.HoldStreams(3)
		s.ImportZvuk(alice)
		<-held

		s.Restart()
		s.Zvuk.ReleaseStreams()
		s.WaitIngest()

		assert.Len(t, s.PersonalFiles(alice), 9)
		for _, id := range collection.Tracks {
			assert.Equal(t, 1, s.Zvuk.DownloadsOf(id), "track %s", id)
		}
	})

	t.Run("followed artists' discographies are not imported", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		liked := s.AddZvukAlbum("640", "Liked", 1)
		discography := s.AddZvukAlbum("650", "Discography", 2)
		s.Zvuk.Update(harness.ZvukToken, func(a *zvuk.Account) {
			a.Liked = liked
			a.Artists = []string{"950"}
		})

		s.ImportZvuk(alice)
		s.WaitIngest()

		assert.Equal(t, []string{"Zvuk Band/Liked (2020)/01 - Song 1.mp3"}, s.PersonalFiles(alice))
		for _, id := range discography {
			assert.Zero(t, s.Zvuk.DownloadsOf(id))
		}
	})
}

func TestImports(t *testing.T) {
	t.Parallel()

	t.Run("started Import opens the Imports", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.Zvuk.HoldStreams(0)

		s.ImportZvuk(alice)

		assert.Contains(t, s.WindowText(), "📊 <b>Импорты</b>")
		assert.Contains(t, s.WindowText(), "⬜⬜⬜⬜⬜⬜⬜⬜⬜⬜ 0 / 9")
	})

	t.Run("Imports follow the progress and drop the finished Import", func(t *testing.T) {
		s := harness.New(t, harness.WithTelegramPollInterval(time.Second))
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.Zvuk.HoldStreams(2)

		s.ImportZvuk(alice)
		require.Eventually(t, func() bool {
			return strings.Contains(s.Telegram.Window().Params["text"], "🟩🟩⬜⬜⬜⬜⬜⬜⬜⬜ 2 / 9")
		}, 5*time.Second, 50*time.Millisecond)
		s.Zvuk.ReleaseStreams()
		s.WaitIngest()

		assert.Contains(t, s.WindowText(), "Сейчас ничего не импортируется")
	})

	t.Run("failed tracks show in red", func(t *testing.T) {
		s := harness.New(t, harness.WithZvukPerUser(2))
		s.ConnectZvuk(alice, harness.ZvukToken)
		collection := s.AddZvukCollection(harness.ZvukToken)
		s.Zvuk.BreakStream(collection.Liked[0])
		s.Zvuk.HoldStreamsOf(collection.Liked[1])

		s.ImportZvuk(alice)

		require.Eventually(t, func() bool {
			return strings.Contains(s.Telegram.Window().Params["text"], "🟩🟩🟩🟩🟩🟩🟩🟩🟥⬜ 7 / 9 · ошибок 1")
		}, 10*time.Second, 20*time.Millisecond)
	})

	t.Run("finished Import sends one summary wherever the window is", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.Zvuk.HoldStreams(0)
		s.ImportZvuk(alice)

		s.Open(alice, "🎵 Лента")
		s.Zvuk.ReleaseStreams()
		s.WaitIngest()

		summaries := s.SentMessagesContaining(alice, "Импорт из Звука")
		require.Len(t, summaries, 1)
		assert.Contains(t, summaries[0], "✅ Импорт из Звука: 9 из 9 в библиотеке")
	})

	t.Run("two instances send one summary", func(t *testing.T) {
		s := harness.New(t, harness.WithLeaseTTL(time.Minute))
		s.StartReplica()
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.Zvuk.HoldStreams(0)
		s.ImportZvuk(alice)
		summary := s.Telegram.Hold("sendMessage")
		sent := len(s.Telegram.CallsTo("sendMessage"))

		s.Zvuk.ReleaseStreams()
		<-summary.Arrived()
		assert.False(t, s.Telegram.WaitCalls("sendMessage", sent+1, anotherRound), "another instance acted meanwhile")
		summary.Release()
		s.WaitIngest()

		assert.Len(t, s.SentMessagesContaining(alice, "Импорт из Звука"), 1)
	})

	t.Run("summary lists the track that failed", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		collection := s.AddZvukCollection(harness.ZvukToken)
		s.Zvuk.BreakStream(collection.Liked[1])

		s.ImportZvuk(alice)
		s.WaitIngest()

		assert.Len(t, s.PersonalFiles(alice), 8)
		summaries := s.SentMessagesContaining(alice, "Импорт из Звука")
		require.Len(t, summaries, 1)
		assert.Contains(t, summaries[0], "8 из 9 в библиотеке")
		assert.Contains(t, summaries[0], "Не удалось загрузить (1)")
		assert.Contains(t, summaries[0], "Zvuk Band — Song 2")
	})

	t.Run("summary Telegram failed to take comes again", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.Zvuk.HoldStreams(0)
		s.ImportZvuk(alice)
		s.Telegram.FailCalls("sendMessage", 1, http.StatusInternalServerError)

		s.Zvuk.ReleaseStreams()
		s.WaitIngest()

		assert.Len(t, s.SentMessagesContaining(alice, "Импорт из Звука"), 1)
	})

	t.Run("restart mid-Import keeps the Imports and the summary", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		held := s.Zvuk.HoldStreams(3)
		s.ImportZvuk(alice)
		<-held

		s.Restart()
		s.Zvuk.ReleaseStreams()
		s.WaitIngest()

		assert.Len(t, s.SentMessagesContaining(alice, "Импорт из Звука"), 1)
		assert.Contains(t, s.WindowText(), "Сейчас ничего не импортируется")
	})

	t.Run("nothing to show without an Import", func(t *testing.T) {
		s := harness.New(t)

		s.Open(alice, "📥 Импорт из музыкального сервиса", "📊 Импорты")

		assert.Contains(t, s.WindowText(), "Сейчас ничего не импортируется")
	})
}

func TestZvukImportPace(t *testing.T) {
	t.Parallel()

	t.Run("Imports of different users download side by side", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.ConnectZvuk(bob, "zvuk-token-bob")
		aliceTracks := s.AddZvukAlbum("710", "Alice Album", 2)
		bobTracks := s.AddZvukAlbum("720", "Bob Album", 2)
		s.Zvuk.Update(harness.ZvukToken, func(a *zvuk.Account) { a.Liked = aliceTracks })
		s.Zvuk.Update("zvuk-token-bob", func(a *zvuk.Account) { a.Liked = bobTracks })
		s.Zvuk.HoldStreams(0)

		s.ImportZvuk(alice)
		s.ImportZvuk(bob)

		require.Eventually(t, func() bool {
			return len(s.Zvuk.HeldTracks()) == 2
		}, 10*time.Second, 10*time.Millisecond)
	})

	t.Run("user downloads as many tracks at once as allowed", func(t *testing.T) {
		s := harness.New(t, harness.WithZvukWorkers(4), harness.WithZvukPerUser(2))
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.Zvuk.HoldStreams(0)

		s.ImportZvuk(alice)

		require.Eventually(t, func() bool {
			return len(s.Zvuk.HeldTracks()) == 2
		}, 10*time.Second, 10*time.Millisecond)
		require.Never(t, func() bool {
			return len(s.Zvuk.HeldTracks()) > 2
		}, 300*time.Millisecond, 10*time.Millisecond)
	})

	t.Run("next track waits out the pause before asking Zvuk for a stream", func(t *testing.T) {
		const pause = 300 * time.Millisecond
		s := harness.New(t, harness.WithZvukPause(pause))
		s.ConnectZvuk(alice, harness.ZvukToken)
		liked := s.AddZvukAlbum("730", "Paced", 2)
		s.Zvuk.Update(harness.ZvukToken, func(a *zvuk.Account) { a.Liked = liked })

		s.ImportZvuk(alice)
		s.WaitIngest()

		asked, served := s.Zvuk.EventsOf(zvuk.StreamAsked), s.Zvuk.EventsOf(zvuk.AudioServed)
		require.Len(t, asked, 2)
		require.Len(t, served, 2)
		assert.GreaterOrEqual(t, asked[1].At.Sub(served[0].At), pause)
	})

	t.Run("one user downloads one track at a time by default", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		held := s.Zvuk.HoldStreams(0)

		s.ImportZvuk(alice)

		<-held
		require.Never(t, func() bool {
			return len(s.Zvuk.HeldTracks()) > 1
		}, 300*time.Millisecond, 10*time.Millisecond)
	})
}
