package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/audiofile"
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
