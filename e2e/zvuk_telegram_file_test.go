package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	app "github.com/lubaskinc0de/navidrome-tg/internal/main"
)

const storageChat = -100500

func TestTelegramFileOfZvukTrack(t *testing.T) {
	t.Parallel()

	t.Run("send file button uploads the file once", func(t *testing.T) {
		s := newScenario(t)
		s.sharedZvukSong(alice, "Uploaded Song")
		s.send(s.textMessage(bob, "/shared"))
		send := buttonNamed(t, s, "1. ▶️ Прислать файл")

		s.press(bob, send)
		s.press(bob, send)

		sent := s.botAPI.callsTo("sendAudio")
		require.Len(t, sent, 2)
		assert.Equal(t, "Uploaded Song.mp3", sent[0].Params[uploadedFileParam])
		assert.Equal(t, "1002", sent[0].Params["chat_id"])
		assert.Equal(t, s.botAPI.uploadedFileID(0), sent[1].Params["audio"])
		assert.Empty(t, sent[1].Params[uploadedFileParam])
	})

	t.Run("file over the upload limit is not sent", func(t *testing.T) {
		s := newScenario(t, withMaxUpload(1024))
		s.sharedZvukSong(alice, "Huge Song")
		s.send(s.textMessage(bob, "/shared"))

		s.press(bob, buttonNamed(t, s, "1. ▶️ Прислать файл"))

		assert.Empty(t, s.botAPI.callsTo("sendAudio"))
		assert.Contains(t, s.botAPI.callbackAnswers(), "Файл слишком большой: Telegram не даёт боту его отправить")
	})

	t.Run("inline shared sends the uploaded file", func(t *testing.T) {
		s := newScenario(t)
		s.sharedZvukSong(alice, "Inline Song")
		s.send(s.textMessage(bob, "/shared"))
		s.press(bob, buttonNamed(t, s, "1. ▶️ Прислать файл"))
		query := s.inlineQuery(bob, "shared")

		s.send(query)

		assert.Equal(t, []string{s.botAPI.uploadedFileID(0)}, audioFileIDs(s.botAPI.inlineAnswerTo(t, query)))
	})

	t.Run("taken track gets the file too", func(t *testing.T) {
		s := newScenario(t)
		s.sharedZvukSong(alice, "Taken Song")
		bobAccount := s.linkNewAccount(bob)
		s.take(bob, 1)
		s.send(s.textMessage(bob, "/shared"))
		s.press(bob, buttonNamed(t, s, "1. ▶️ Прислать файл"))
		song := env.navidrome.indexedTrack(t, bobAccount, s.personalPath(bob, ""), "Taken Song")
		env.navidrome.startPlaying(t, bobAccount, song.ID)
		query := s.inlineQuery(bob, "np")

		s.send(query)

		assert.Equal(t, []string{s.botAPI.uploadedFileID(0)}, audioFileIDs(s.botAPI.inlineAnswerTo(t, query)))
	})

	t.Run("storage chat gets the file right after Ingest", func(t *testing.T) {
		s := newScenario(t, withStorageChat(storageChat))
		s.connectZvuk(alice, zvukToken)
		s.addZvukSong("111", "Stored Song", false)
		s.likeOnZvuk("111")

		s.importZvuk(alice)
		s.waitIngest()

		sent := s.botAPI.callsTo("sendAudio")
		require.Len(t, sent, 1)
		assert.Equal(t, "-100500", sent[0].Params["chat_id"])
		assert.Equal(t, "Stored Song.mp3", sent[0].Params[uploadedFileParam])
	})

	t.Run("np sends a track kept in the storage chat as audio", func(t *testing.T) {
		s := newScenario(t, withStorageChat(storageChat))
		account := s.linkNewAccount(alice)
		s.connectZvuk(alice, zvukToken)
		s.addZvukSong("111", "Stored Song", false)
		s.likeOnZvuk("111")
		s.importZvuk(alice)
		s.waitIngest()
		song := env.navidrome.indexedTrack(t, account, s.personalPath(alice, ""), "Stored Song")
		env.navidrome.startPlaying(t, account, song.ID)
		query := s.inlineQuery(alice, "np")

		s.send(query)

		assert.Equal(t, []string{s.botAPI.uploadedFileID(0)}, audioFileIDs(s.botAPI.inlineAnswerTo(t, query)))
	})
}

func withStorageChat(chatID int64) scenarioOption {
	return func(c *app.Config) { c.StorageChatID = chatID }
}

// sharedZvukSong has the user import a liked Zvuk track and share it
// from np, the only place a track without an audio message offers Share.
func (s *scenario) sharedZvukSong(user telegramUser, title string) {
	s.t.Helper()

	account := s.linkNewAccount(user)
	s.connectZvuk(user, zvukToken)
	s.addZvukSong("110", title, false)
	s.likeOnZvuk("110")
	s.importZvuk(user)
	s.waitIngest()
	song := env.navidrome.indexedTrack(s.t, account, s.personalPath(user, ""), title)
	env.navidrome.startPlaying(s.t, account, song.ID)

	s.pressInline(user, buttonIn(s.t, s.nowPlayingButtons(user), "🔗 Share"))
	require.Len(s.t, s.sharedFiles(), 1)
}

func withMaxUpload(bytes int64) scenarioOption {
	return func(c *app.Config) { c.MaxPostSize = bytes }
}
