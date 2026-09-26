package e2e

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
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

		s.Send(s.TextMessage(alice, "/zvuk_import"))

		text := s.LastReply().Text
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

		s.Send(s.TextMessage(alice, "/zvuk_import"))

		assert.Contains(t, s.LastReply().Text, "В коллекции Звука 10 треков, в библиотеке ещё нет 1 трек")
	})

	t.Run("repeated Import of a stored collection offers nothing", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.ImportZvuk(alice)
		s.WaitIngest()

		s.Send(s.TextMessage(alice, "/zvuk_import"))

		assert.Contains(t, s.LastReply().Text, "Вся коллекция Звука (9 треков) уже в вашей библиотеке")
	})

	t.Run("button of an old offer tells the collection is stored", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.Send(s.TextMessage(alice, "/zvuk_import"))
		start := s.Button("▶️ Начать Import")
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
		s.Send(s.TextMessage(alice, "/zvuk_import"))
		start := s.Button("▶️ Начать Import")
		s.Press(alice, start)

		s.Press(alice, start)

		assert.Contains(t, s.Telegram.CallbackAnswers(), "Import уже идёт")
		assert.Len(t, s.SentMessagesContaining("Import из Звука"), 1)
	})

	t.Run("progress message is edited into a summary", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)

		s.ImportZvuk(alice)
		s.WaitIngest()

		progress := s.ProgressMessage(t)
		assert.Contains(t, progress.Text, "Import из Звука: 9 из 9 треков в библиотеке")
		assert.Greater(t, progress.Edits, 1)
		assert.Len(t, s.Telegram.EditedMessages(), 1)
		assert.Len(t, s.SentMessagesContaining("Import из Звука"), 1)
	})

	t.Run("progress catches up while the next track is on its way", func(t *testing.T) {
		s := harness.New(t, harness.WithTelegramPollInterval(time.Second))
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.Zvuk.HoldStreams(2)

		s.ImportZvuk(alice)

		require.Eventually(t, func() bool {
			edits := s.Telegram.CallsTo("editMessageText")
			return len(edits) > 0 && strings.Contains(edits[len(edits)-1].Params["text"], "готово 2 из 9")
		}, 5*time.Second, 50*time.Millisecond)
	})

	t.Run("summary lists the track that failed", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		collection := s.AddZvukCollection(harness.ZvukToken)
		s.Zvuk.BreakStream(collection.Liked[1])

		s.ImportZvuk(alice)
		s.WaitIngest()

		assert.Len(t, s.PersonalFiles(alice), 8)
		text := s.ProgressMessage(t).Text
		assert.Contains(t, text, "8 из 9 треков в библиотеке")
		assert.Contains(t, text, "Не удалось загрузить (1)")
		assert.Contains(t, text, "Zvuk Band — Song 2")
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
		assert.Contains(t, s.ProgressMessage(t).Text, "9 из 9 треков в библиотеке")
	})

	t.Run("progress after restart reaches the summary in the same message", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		held := s.Zvuk.HoldStreams(3)
		s.ImportZvuk(alice)
		<-held

		s.Restart()
		s.Zvuk.ReleaseStreams()
		s.WaitIngest()

		assert.Len(t, s.SentMessagesContaining("Import из Звука"), 1)
		assert.Len(t, s.Telegram.EditedMessages(), 1)
		assert.Contains(t, s.ProgressMessage(t).Text, "9 из 9 треков в библиотеке")
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

	t.Run("Import without connected Zvuk suggests connecting it", func(t *testing.T) {
		s := harness.New(t)

		s.Send(s.TextMessage(alice, "/zvuk_import"))

		assert.Contains(t, s.LastReply().Text, "Подключите Звук")
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
