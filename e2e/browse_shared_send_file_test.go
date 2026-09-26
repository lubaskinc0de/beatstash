package e2e

import (
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/telegram"
)

const storageChat = -100500

func TestTelegramFileOfZvukTrack(t *testing.T) {
	t.Parallel()

	t.Run("send file button uploads the file once", func(t *testing.T) {
		s := harness.New(t)
		sharedZvukSong(t, s, alice, "Uploaded Song")
		s.Open(bob, "🎵 Лента")
		send := s.Button("1. ▶️ Прислать файл")

		s.Press(bob, send)
		s.Press(bob, send)

		sent := s.Telegram.CallsTo("sendAudio")
		require.Len(t, sent, 2)
		assert.Equal(t, "Uploaded Song.mp3", sent[0].Params[telegram.UploadedFileParam])
		assert.Equal(t, "1002", sent[0].Params["chat_id"])
		assert.Equal(t, s.Telegram.UploadedFileID(0), sent[1].Params["audio"])
		assert.Empty(t, sent[1].Params[telegram.UploadedFileParam])
	})

	t.Run("file over the upload limit is not sent", func(t *testing.T) {
		s := harness.New(t, harness.WithMaxUpload(1024))
		sharedZvukSong(t, s, alice, "Huge Song")
		s.Open(bob, "🎵 Лента")

		s.Press(bob, s.Button("1. ▶️ Прислать файл"))

		assert.Empty(t, s.Telegram.CallsTo("sendAudio"))
		assert.Contains(t, s.Telegram.CallbackAnswers(), "Файл слишком большой: Telegram не даёт боту его отправить")
	})

	t.Run("inline shared sends the uploaded file", func(t *testing.T) {
		s := harness.New(t)
		sharedZvukSong(t, s, alice, "Inline Song")
		s.Open(bob, "🎵 Лента")
		s.Press(bob, s.Button("1. ▶️ Прислать файл"))
		query := s.InlineQuery(bob, "shared")

		s.Send(query)

		assert.Equal(t, []string{s.Telegram.UploadedFileID(0)}, telegram.AudioFileIDs(s.Telegram.InlineAnswerTo(t, query)))
	})

	t.Run("taken track gets the file too", func(t *testing.T) {
		s := harness.New(t)
		sharedZvukSong(t, s, alice, "Taken Song")
		bobAccount := s.LinkNewAccount(bob)
		s.Take(bob, 1)
		s.Open(bob, "🎵 Лента")
		s.Press(bob, s.Button("1. ▶️ Прислать файл"))
		song := s.Navidrome.IndexedTrack(t, bobAccount, s.PersonalPath(bob, ""), "Taken Song")
		s.Navidrome.StartPlaying(t, bobAccount, song.ID)
		query := s.InlineQuery(bob, "np")

		s.Send(query)

		assert.Equal(t, []string{s.Telegram.UploadedFileID(0)}, telegram.AudioFileIDs(s.Telegram.InlineAnswerTo(t, query)))
	})

	t.Run("storage chat gets the file right after Ingest", func(t *testing.T) {
		s := harness.New(t, harness.WithStorageChat(storageChat))
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukSong("111", "Stored Song", false)
		s.LikeOnZvuk("111")

		s.ImportZvuk(alice)
		s.WaitIngest()

		sent := s.Telegram.CallsTo("sendAudio")
		require.Len(t, sent, 1)
		assert.Equal(t, "-100500", sent[0].Params["chat_id"])
		assert.Equal(t, "Stored Song.mp3", sent[0].Params[telegram.UploadedFileParam])
	})

	t.Run("audio the bot sent from the feed is stored without download", func(t *testing.T) {
		s := harness.New(t)
		sharedZvukSong(t, s, alice, "Forwarded Song")
		s.Open(bob, "🎵 Лента")
		s.Press(bob, s.Button("1. ▶️ Прислать файл"))
		s.Telegram.Forget()
		forward := s.AudioMessage(bob, uploadedAudio(s, 0))

		s.Send(forward)
		s.WaitIngest()

		assert.Empty(t, s.Telegram.CallsTo("getFile"))
		assert.Equal(t, []string{"Zvuk Band/Zvuk Album (2021)/01 - Forwarded Song.mp3"}, s.PersonalFiles(bob))
		assert.Equal(t, []string{"👀", "👍"}, s.Telegram.ReactionsOn(t, forward.Message.ID))
	})

	t.Run("audio the bot sent of the user's own track already exists", func(t *testing.T) {
		s := harness.New(t, harness.WithStorageChat(storageChat))
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukSong("111", "Stored Song", false)
		s.LikeOnZvuk("111")
		s.ImportZvuk(alice)
		s.WaitIngest()
		s.Telegram.Forget()
		forward := s.AudioMessage(alice, uploadedAudio(s, 0))

		s.Send(forward)
		s.WaitIngest()

		assert.Empty(t, s.Telegram.CallsTo("getFile"))
		assert.Equal(t, []string{"Zvuk Band/Zvuk Album (2021)/01 - Stored Song.mp3"}, s.PersonalFiles(alice))
		harness.AssertAlreadyExists(t, s, forward.Message.ID)
	})

	t.Run("np sends a track kept in the storage chat as audio", func(t *testing.T) {
		s := harness.New(t, harness.WithStorageChat(storageChat))
		account := s.LinkNewAccount(alice)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukSong("111", "Stored Song", false)
		s.LikeOnZvuk("111")
		s.ImportZvuk(alice)
		s.WaitIngest()
		song := s.Navidrome.IndexedTrack(t, account, s.PersonalPath(alice, ""), "Stored Song")
		s.Navidrome.StartPlaying(t, account, song.ID)
		query := s.InlineQuery(alice, "np")

		s.Send(query)

		assert.Equal(t, []string{s.Telegram.UploadedFileID(0)}, telegram.AudioFileIDs(s.Telegram.InlineAnswerTo(t, query)))
	})
}

func uploadedAudio(s *harness.Scenario, n int) models.Audio {
	id := s.Telegram.UploadedFileID(n)
	return models.Audio{FileID: id, FileUniqueID: id + "-unique", FileName: "song.mp3", Duration: 2}
}

// sharedZvukSong has the user import a liked Zvuk track and share it
// from np, the only place a track without an audio message offers Share.
func sharedZvukSong(t *testing.T, s *harness.Scenario, user harness.User, title string) {
	t.Helper()

	account := s.LinkNewAccount(user)
	s.ConnectZvuk(user, harness.ZvukToken)
	s.AddZvukSong("110", title, false)
	s.LikeOnZvuk("110")
	s.ImportZvuk(user)
	s.WaitIngest()
	song := s.Navidrome.IndexedTrack(t, account, s.PersonalPath(user, ""), title)
	s.Navidrome.StartPlaying(t, account, song.ID)

	s.PressInline(user, telegram.ButtonNamed(t, s.NowPlayingButtons(user), "🔗 Поделиться"))
	require.Len(t, s.SharedFiles(), 1)
}
