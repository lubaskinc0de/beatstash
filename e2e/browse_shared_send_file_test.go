package e2e

import (
	"testing"

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
		s.Send(s.TextMessage(bob, "/shared"))
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
		s.Send(s.TextMessage(bob, "/shared"))

		s.Press(bob, s.Button("1. ▶️ Прислать файл"))

		assert.Empty(t, s.Telegram.CallsTo("sendAudio"))
		assert.Contains(t, s.Telegram.CallbackAnswers(), "Файл слишком большой: Telegram не даёт боту его отправить")
	})

	t.Run("inline shared sends the uploaded file", func(t *testing.T) {
		s := harness.New(t)
		sharedZvukSong(t, s, alice, "Inline Song")
		s.Send(s.TextMessage(bob, "/shared"))
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
		s.Send(s.TextMessage(bob, "/shared"))
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

	s.PressInline(user, telegram.ButtonNamed(t, s.NowPlayingButtons(user), "🔗 Share"))
	require.Len(t, s.SharedFiles(), 1)
}
