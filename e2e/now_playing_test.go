package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNowPlaying(t *testing.T) {
	t.Run("np returns the track playing in Navidrome", func(t *testing.T) {
		s := newScenario(t)
		audio := s.uploadAudio("track.mp3")
		s.send(s.audioMessage(allowedUser, audio))
		track := env.navidrome.indexedTrack(t, s.library, fixtureTitle)
		env.navidrome.startPlaying(t, track.ID)
		query := s.inlineQuery(allowedUser, "np")

		s.send(query)

		answers := s.botAPI.InlineAnswers(t)
		require.Len(t, answers, 1)
		assert.Equal(t, query.InlineQuery.ID, answers[0].QueryID)
		require.Len(t, answers[0].Results, 1)
		assert.Equal(t, "audio", answers[0].Results[0].Type)
		assert.Equal(t, audio.FileID, answers[0].Results[0].AudioFileID)
		assert.Contains(t, answers[0].Results[0].Caption, fixtureTitle)
	})
}
