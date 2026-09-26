package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/audiofile"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/telegram"
)

func TestNowPlaying(t *testing.T) {
	t.Parallel()

	t.Run("np returns the track playing in Navidrome", func(t *testing.T) {
		s := harness.New(t)
		account := s.Navidrome.CreateAccount(t, "alice")
		s.Link(alice, account)
		audio := s.UploadAudio("track.mp3")
		s.Send(s.AudioMessage(alice, audio))
		track := s.Navidrome.IndexedTrack(t, account, s.Library, audiofile.FixtureTitle)
		s.Navidrome.StartPlaying(t, account, track.ID)
		query := s.InlineQuery(alice, "np")

		s.Send(query)

		answers := s.Telegram.InlineAnswers(t)
		require.Len(t, answers, 1)
		assert.Equal(t, query.InlineQuery.ID, answers[0].QueryID)
		require.Len(t, answers[0].Results, 1)
		assert.Equal(t, "audio", answers[0].Results[0].Type)
		assert.Equal(t, audio.FileID, answers[0].Results[0].AudioFileID)
		assert.Contains(t, answers[0].Results[0].Caption, audiofile.FixtureTitle)
	})
}

func TestInlineHints(t *testing.T) {
	t.Parallel()

	t.Run("empty query teaches the inline commands", func(t *testing.T) {
		s := harness.New(t)
		query := s.InlineQuery(alice, "")

		s.Send(query)

		results := s.Telegram.InlineAnswerTo(t, query).Results
		require.Len(t, results, 4)
		for n, command := range []string{"np", "recent", "shared", "top"} {
			assert.Contains(t, results[n].Title, command)
			assert.Contains(t, results[n].Content.Text, "@"+telegram.BotUsername+" "+command)
			buttons := results[n].Buttons()
			require.Len(t, buttons, 1)
			assert.Equal(t, command, buttons[0].SwitchInline)
		}
	})

	t.Run("unknown command teaches the inline commands too", func(t *testing.T) {
		s := harness.New(t)
		query := s.InlineQuery(alice, "what")

		s.Send(query)

		assert.Len(t, s.Telegram.InlineAnswerTo(t, query).Results, 4)
	})

	t.Run("hints follow the user's language", func(t *testing.T) {
		s := harness.New(t)
		s.Open(alice, "🌐 English")
		query := s.InlineQuery(alice, "")

		s.Send(query)

		assert.Equal(t, "Now playing", s.Telegram.InlineAnswerTo(t, query).Results[0].Description)
	})
}
