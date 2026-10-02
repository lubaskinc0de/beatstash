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
		s.WaitIngest()
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

	t.Run("np explains that nothing is playing for a linked account", func(t *testing.T) {
		s := harness.New(t)
		s.LinkNewAccount(alice)
		article := s.Catalog(alice).NothingPlaying()
		query := s.InlineQuery(alice, "np")

		s.Send(query)

		results := s.Telegram.InlineAnswerTo(t, query).Results
		require.Len(t, results, 1)
		assert.Contains(t, results[0].Title, article.Title)
		assert.Contains(t, results[0].Content.Text, article.Message)
	})

	t.Run("recent explains that the linked account has no history", func(t *testing.T) {
		s := harness.New(t)
		s.LinkNewAccount(alice)
		article := s.Catalog(alice).HistoryEmpty()
		query := s.InlineQuery(alice, "recent")

		s.Send(query)

		results := s.Telegram.InlineAnswerTo(t, query).Results
		require.Len(t, results, 1)
		assert.Contains(t, results[0].Title, article.Title)
		assert.Contains(t, results[0].Content.Text, article.Message)
	})
}

func TestInlineHints(t *testing.T) {
	t.Parallel()

	t.Run("empty query teaches the inline commands", func(t *testing.T) {
		s := harness.New(t)
		query := s.InlineQuery(alice, "")

		s.Send(query)

		results := s.Telegram.InlineAnswerTo(t, query).Results
		require.Len(t, results, 5)
		assert.Equal(t, s.Catalog(alice).SearchHint(telegram.BotUsername).Description, results[4].Description)
		for n, command := range []string{"np", "recent", "shared", "top"} {
			assert.Contains(t, results[n].Title, command)
			assert.Contains(t, results[n].Content.Text, "@"+telegram.BotUsername+" "+command)
			buttons := results[n].Buttons()
			require.Len(t, buttons, 1)
			assert.Equal(t, command, buttons[0].SwitchInline)
		}
	})

	t.Run("command stays a command", func(t *testing.T) {
		s := harness.New(t)
		s.Uploaded(alice, s.UploadAudioFile(audiofile.Generate(t, "top.mp3", audiofile.Spec{Tags: map[string]string{
			"artist": "Top", "album": "Top", "title": "Top", "track": "1",
		}})))

		results := s.Search(alice, " TOP ", "").Results

		require.Len(t, results, 1)
		assert.Equal(t, "top", results[0].ID)
	})

	t.Run("hints follow the user's language", func(t *testing.T) {
		s := harness.New(t)
		s.Open(alice, s.Catalog(alice).SettingsButton(), s.Catalog(harness.User{LanguageCode: "en"}).LanguageButton())
		query := s.InlineQuery(alice, "")

		s.Send(query)

		english := alice
		english.LanguageCode = "en"
		assert.Equal(t, s.Catalog(english).Hints(telegram.BotUsername)[0].Description, s.Telegram.InlineAnswerTo(t, query).Results[0].Description)
	})
}
