package e2e

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	app "github.com/lubaskinc0de/navidrome-tg/internal/main"
)

func TestZvukImport(t *testing.T) {
	t.Parallel()

	t.Run("Import brings each track of the collection once", func(t *testing.T) {
		s := newScenario(t)
		s.connectZvuk(alice, zvukToken)
		s.addZvukCollection(zvukToken)

		s.importZvuk(alice)
		s.waitIngest()

		files := s.personalFiles(alice)
		assert.Len(t, files, 9)
		assert.Contains(t, files, "Zvuk Band/Saved (2020)/01 - Song 1.mp3")
		assert.Contains(t, files, "Zvuk Band/Liked (2020)/05 - Song 5.mp3")
		assert.Contains(t, files, "Zvuk Band/Listed (2020)/01 - Song 1.mp3")
	})

	t.Run("bot tells the size before the Import starts", func(t *testing.T) {
		s := newScenario(t)
		s.connectZvuk(alice, zvukToken)
		s.addZvukCollection(zvukToken)

		s.send(s.textMessage(alice, "/zvuk_import"))

		text := lastReply(t, s).Text
		assert.Contains(t, text, "9 треков")
		assert.Contains(t, text, "МБ")
		assert.Empty(t, s.personalFiles(alice))
	})

	t.Run("repeated Import offers only the tracks the library lacks", func(t *testing.T) {
		s := newScenario(t)
		s.connectZvuk(alice, zvukToken)
		s.addZvukCollection(zvukToken)
		s.importZvuk(alice)
		s.waitIngest()
		fresh := s.addZvukAlbum("675", "Fresh", 1)
		s.zvuk.update(zvukToken, func(a *zvukAccount) { a.liked = append(a.liked, fresh...) })

		s.send(s.textMessage(alice, "/zvuk_import"))

		assert.Contains(t, lastReply(t, s).Text, "В коллекции Звука 10 треков, в библиотеке ещё нет 1 трек")
	})

	t.Run("repeated Import of a stored collection offers nothing", func(t *testing.T) {
		s := newScenario(t)
		s.connectZvuk(alice, zvukToken)
		s.addZvukCollection(zvukToken)
		s.importZvuk(alice)
		s.waitIngest()

		s.send(s.textMessage(alice, "/zvuk_import"))

		assert.Contains(t, lastReply(t, s).Text, "Вся коллекция Звука (9 треков) уже в вашей библиотеке")
	})

	t.Run("button of an old offer tells the collection is stored", func(t *testing.T) {
		s := newScenario(t)
		s.connectZvuk(alice, zvukToken)
		s.addZvukCollection(zvukToken)
		s.send(s.textMessage(alice, "/zvuk_import"))
		start := buttonNamed(t, s, "▶️ Начать Import")
		s.press(alice, start)
		s.waitIngest()

		s.press(alice, start)

		assert.Contains(t, s.botAPI.callbackAnswers(), "Вся коллекция уже в библиотеке")
	})

	t.Run("second tap on the button starts no second Import", func(t *testing.T) {
		s := newScenario(t)
		s.connectZvuk(alice, zvukToken)
		s.addZvukCollection(zvukToken)
		s.zvuk.holdStreams(0)
		s.send(s.textMessage(alice, "/zvuk_import"))
		start := buttonNamed(t, s, "▶️ Начать Import")
		s.press(alice, start)

		s.press(alice, start)

		assert.Contains(t, s.botAPI.callbackAnswers(), "Import уже идёт")
		assert.Len(t, s.sentMessagesContaining("Import из Звука"), 1)
	})

	t.Run("progress message is edited into a summary", func(t *testing.T) {
		s := newScenario(t)
		s.connectZvuk(alice, zvukToken)
		s.addZvukCollection(zvukToken)

		s.importZvuk(alice)
		s.waitIngest()

		progress := s.progressMessage(t)
		assert.Contains(t, progress.Text, "Import из Звука: 9 из 9 треков в библиотеке")
		assert.Greater(t, progress.Edits, 1)
		assert.Len(t, s.botAPI.editedMessages(), 1)
		assert.Len(t, s.sentMessagesContaining("Import из Звука"), 1)
	})

	t.Run("progress catches up while the next track is on its way", func(t *testing.T) {
		s := newScenario(t, withProgressInterval(time.Second))
		s.connectZvuk(alice, zvukToken)
		s.addZvukCollection(zvukToken)
		s.zvuk.holdStreams(2)

		s.importZvuk(alice)

		require.Eventually(t, func() bool {
			edits := s.botAPI.callsTo("editMessageText")
			return len(edits) > 0 && strings.Contains(edits[len(edits)-1].Params["text"], "готово 2 из 9")
		}, 5*time.Second, 50*time.Millisecond)
	})

	t.Run("summary lists the track that failed", func(t *testing.T) {
		s := newScenario(t)
		s.connectZvuk(alice, zvukToken)
		collection := s.addZvukCollection(zvukToken)
		s.zvuk.breakStream(collection.liked[1])

		s.importZvuk(alice)
		s.waitIngest()

		assert.Len(t, s.personalFiles(alice), 8)
		text := s.progressMessage(t).Text
		assert.Contains(t, text, "8 из 9 треков в библиотеке")
		assert.Contains(t, text, "Не удалось загрузить (1)")
		assert.Contains(t, text, "Zvuk Band — Song 2")
	})

	t.Run("interrupted Import finishes after restart without downloading twice", func(t *testing.T) {
		s := newScenario(t)
		s.connectZvuk(alice, zvukToken)
		collection := s.addZvukCollection(zvukToken)
		held := s.zvuk.holdStreams(3)
		s.importZvuk(alice)
		<-held

		s.restart()
		s.zvuk.releaseStreams()
		s.waitIngest()

		assert.Len(t, s.personalFiles(alice), 9)
		for _, id := range collection.tracks {
			assert.Equal(t, 1, s.zvuk.downloadsOf(id), "track %s", id)
		}
		assert.Contains(t, s.progressMessage(t).Text, "9 из 9 треков в библиотеке")
	})

	t.Run("followed artists' discographies are not imported", func(t *testing.T) {
		s := newScenario(t)
		s.connectZvuk(alice, zvukToken)
		liked := s.addZvukAlbum("640", "Liked", 1)
		discography := s.addZvukAlbum("650", "Discography", 2)
		s.zvuk.update(zvukToken, func(a *zvukAccount) {
			a.liked = liked
			a.artists = []string{"950"}
		})

		s.importZvuk(alice)
		s.waitIngest()

		assert.Equal(t, []string{"Zvuk Band/Liked (2020)/01 - Song 1.mp3"}, s.personalFiles(alice))
		for _, id := range discography {
			assert.Zero(t, s.zvuk.downloadsOf(id))
		}
	})

	t.Run("Import without connected Zvuk suggests connecting it", func(t *testing.T) {
		s := newScenario(t)

		s.send(s.textMessage(alice, "/zvuk_import"))

		assert.Contains(t, lastReply(t, s).Text, "Подключите Звук")
	})
}

type zvukCollection struct {
	liked    []string
	album    []string
	playlist []string
	// tracks lists each track of the collection once.
	tracks []string
}

// addZvukCollection saves into the account 5 liked tracks, an album of 3
// and a playlist of 2, one of them liked: 9 tracks in all.
func (s *scenario) addZvukCollection(token string) zvukCollection {
	s.t.Helper()

	var c zvukCollection
	c.liked = s.addZvukAlbum("620", "Liked", 5)
	c.album = s.addZvukAlbum("610", "Saved", 3)
	listed := s.addZvukAlbum("630", "Listed", 1)
	c.playlist = []string{listed[0], c.liked[0]}
	s.zvuk.addPlaylist(zvukPlaylist{ID: "810", Title: "My Playlist", Tracks: c.playlist})
	s.zvuk.update(token, func(a *zvukAccount) {
		a.liked = c.liked
		a.releases = []string{"610"}
		a.playlists = []string{"810"}
	})

	c.tracks = append(append(append(c.tracks, c.liked...), c.album...), listed[0])
	return c
}

func (s *scenario) importZvuk(user telegramUser) {
	s.t.Helper()

	s.send(s.textMessage(user, "/zvuk_import"))
	s.press(user, buttonNamed(s.t, s, "▶️ Начать Import"))
}

func (s *scenario) sentMessagesContaining(text string) []string {
	var texts []string
	for _, call := range s.botAPI.callsTo("sendMessage") {
		if strings.Contains(call.Params["text"], text) {
			texts = append(texts, call.Params["text"])
		}
	}
	return texts
}

func (a *botAPI) editedMessages() []string {
	var ids []string
	for _, call := range a.callsTo("editMessageText") {
		id := call.Params["message_id"]
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	return ids
}

func withZvukWorkers(n int) scenarioOption {
	return func(c *app.Config) { c.ZvukWorkers = n }
}

func withZvukPerUser(n int) scenarioOption {
	return func(c *app.Config) { c.ZvukPerUser = n }
}

func withProgressInterval(d time.Duration) scenarioOption {
	return func(c *app.Config) { c.ProgressInterval = d }
}

func withZvukPause(d time.Duration) scenarioOption {
	return func(c *app.Config) { c.ZvukPauseMin, c.ZvukPauseMax = d, d }
}

func TestZvukImportPace(t *testing.T) {
	t.Parallel()

	t.Run("Imports of different users download side by side", func(t *testing.T) {
		s := newScenario(t)
		s.connectZvuk(alice, zvukToken)
		s.connectZvuk(bob, "zvuk-token-bob")
		aliceTracks := s.addZvukAlbum("710", "Alice Album", 2)
		bobTracks := s.addZvukAlbum("720", "Bob Album", 2)
		s.zvuk.update(zvukToken, func(a *zvukAccount) { a.liked = aliceTracks })
		s.zvuk.update("zvuk-token-bob", func(a *zvukAccount) { a.liked = bobTracks })
		s.zvuk.holdStreams(0)

		s.importZvuk(alice)
		s.importZvuk(bob)

		require.Eventually(t, func() bool {
			return len(s.zvuk.heldTracks()) == 2
		}, 10*time.Second, 10*time.Millisecond)
	})

	t.Run("user downloads as many tracks at once as allowed", func(t *testing.T) {
		s := newScenario(t, withZvukWorkers(4), withZvukPerUser(2))
		s.connectZvuk(alice, zvukToken)
		s.addZvukCollection(zvukToken)
		s.zvuk.holdStreams(0)

		s.importZvuk(alice)

		require.Eventually(t, func() bool {
			return len(s.zvuk.heldTracks()) == 2
		}, 10*time.Second, 10*time.Millisecond)
		require.Never(t, func() bool {
			return len(s.zvuk.heldTracks()) > 2
		}, 300*time.Millisecond, 10*time.Millisecond)
	})

	t.Run("next track waits out the pause before asking Zvuk for a stream", func(t *testing.T) {
		const pause = 300 * time.Millisecond
		s := newScenario(t, withZvukPause(pause))
		s.connectZvuk(alice, zvukToken)
		liked := s.addZvukAlbum("730", "Paced", 2)
		s.zvuk.update(zvukToken, func(a *zvukAccount) { a.liked = liked })

		s.importZvuk(alice)
		s.waitIngest()

		asked, served := s.zvuk.eventsOf(streamAsked), s.zvuk.eventsOf(audioServed)
		require.Len(t, asked, 2)
		require.Len(t, served, 2)
		assert.GreaterOrEqual(t, asked[1].at.Sub(served[0].at), pause)
	})

	t.Run("one user downloads one track at a time by default", func(t *testing.T) {
		s := newScenario(t)
		s.connectZvuk(alice, zvukToken)
		s.addZvukCollection(zvukToken)
		held := s.zvuk.holdStreams(0)

		s.importZvuk(alice)

		<-held
		require.Never(t, func() bool {
			return len(s.zvuk.heldTracks()) > 1
		}, 300*time.Millisecond, 10*time.Millisecond)
	})
}
