package e2e

import (
	"sync"
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/navidrome"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/telegram"
)

func TestNowPlayingAudio(t *testing.T) {
	t.Parallel()

	t.Run("chosen np turns into audio and the next np is audio", func(t *testing.T) {
		s := harness.New(t, harness.WithStorageChat(storageChat), harness.WithoutStorageFill())
		playingUnpostedZvukSong(t, s, alice, "Pending Song")
		result := s.NowPlaying(alice)

		chosen := s.Choose(alice, result)
		next := s.NowPlaying(alice)

		assert.Equal(t, "article", result.Type)
		assert.Contains(t, result.Content.Text, "⏳")
		edits := s.Telegram.InlineEdits(t, chosen.ChosenInlineResult.InlineMessageID)
		require.Len(t, edits, 1)
		assert.Equal(t, "editMessageMedia", edits[0].Method)
		assert.Equal(t, "audio", edits[0].Media.Type)
		assert.Equal(t, s.Telegram.UploadedFileID(0), edits[0].Media.Media)
		assert.Contains(t, edits[0].Media.Caption, "Pending Song")
		assert.NotContains(t, edits[0].Media.Caption, "⏳")
		assert.Equal(t, []string{"🔗 Поделиться"}, telegram.ButtonTexts(edits[0].Buttons))
		assert.Equal(t, "audio", next.Type)
		assert.Equal(t, s.Telegram.UploadedFileID(0), next.AudioFileID)
	})

	t.Run("file over the upload limit leaves text with a note", func(t *testing.T) {
		s := harness.New(t, harness.WithStorageChat(storageChat), harness.WithoutStorageFill(), harness.WithMaxUpload(1024))
		playingUnpostedZvukSong(t, s, alice, "Huge Song")

		chosen := s.Choose(alice, s.NowPlaying(alice))

		edits := s.Telegram.InlineEdits(t, chosen.ChosenInlineResult.InlineMessageID)
		require.Len(t, edits, 1)
		assert.Equal(t, "editMessageText", edits[0].Method)
		assert.Contains(t, edits[0].Text, "Huge Song")
		assert.Contains(t, edits[0].Text, "Файл слишком большой")
		assert.NotContains(t, edits[0].Text, "⏳")
	})

	t.Run("chosen recent track turns into audio", func(t *testing.T) {
		s := harness.New(t, harness.WithStorageChat(storageChat), harness.WithoutStorageFill())
		account, song := unpostedZvukSong(t, s, alice, "Played Song")
		s.Navidrome.Play(t, account, song.ID)
		query := s.InlineQuery(alice, "recent")
		s.Send(query)
		results := s.Telegram.InlineAnswerTo(t, query).Results
		require.Len(t, results, 2)
		result := results[1]

		chosen := s.Choose(alice, result)

		assert.Contains(t, result.Content.Text, "⏳")
		assert.Equal(t, []string{"⏳"}, telegram.ButtonTexts(result.Buttons()))
		edits := s.Telegram.InlineEdits(t, chosen.ChosenInlineResult.InlineMessageID)
		require.Len(t, edits, 1)
		assert.Equal(t, "editMessageMedia", edits[0].Method)
		assert.Equal(t, s.Telegram.UploadedFileID(0), edits[0].Media.Media)
		assert.Empty(t, edits[0].Buttons)
	})

	t.Run("simultaneous choices upload the file once", func(t *testing.T) {
		s := harness.New(t, harness.WithStorageChat(storageChat), harness.WithoutStorageFill())
		playingUnpostedZvukSong(t, s, alice, "Pending Song")
		result := s.NowPlaying(alice)
		choices := []*models.Update{s.ChosenResult(alice, result), s.ChosenResult(alice, result), s.ChosenResult(alice, result)}

		var wg sync.WaitGroup
		for _, choice := range choices {
			wg.Go(func() { s.Send(choice) })
		}
		wg.Wait()

		assert.Empty(t, s.Telegram.UploadedFileID(1))
		for _, choice := range choices {
			edits := s.Telegram.InlineEdits(t, choice.ChosenInlineResult.InlineMessageID)
			require.Len(t, edits, 1)
			assert.Equal(t, s.Telegram.UploadedFileID(0), edits[0].Media.Media)
		}
	})
}

func playingUnpostedZvukSong(t *testing.T, s *harness.Scenario, user harness.User, title string) {
	t.Helper()

	account, song := unpostedZvukSong(t, s, user, title)
	s.Navidrome.StartPlaying(t, account, song.ID)
}

// unpostedZvukSong has the user import a Zvuk track; with the storage chat
// filled only on demand it has no Telegram file yet.
func unpostedZvukSong(t *testing.T, s *harness.Scenario, user harness.User, title string) (navidrome.Account, navidrome.Track) {
	t.Helper()

	account := s.LinkNewAccount(user)
	s.ConnectZvuk(user, harness.ZvukToken)
	s.AddZvukSong("111", title, false)
	s.LikeOnZvuk("111")
	s.ImportZvuk(user)
	s.WaitIngest()
	return account, s.Navidrome.IndexedTrack(t, account, s.PersonalPath(user, ""), title)
}
