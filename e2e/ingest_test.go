package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIngest(t *testing.T) {
	t.Parallel()

	t.Run("tagged MP3 lands at artist/album/track path", func(t *testing.T) {
		s := newScenario(t)
		audio := s.uploadAudio("track.mp3")

		s.send(s.audioMessage(alice, audio))
		s.waitIngest()

		assert.Equal(t, []string{"👀", "👍"}, s.botAPI.reactions(t))
		assert.Equal(t, []string{fixtureTrackPath}, s.personalFiles(alice))
	})

	t.Run("audio is acknowledged before Ingest", func(t *testing.T) {
		s := newScenario(t, withoutWorkers())
		audio := s.uploadAudio("track.mp3")

		s.send(s.audioMessage(alice, audio))

		assert.Equal(t, []string{"👀"}, s.botAPI.reactions(t))
		assert.Empty(t, s.botAPI.callsTo("getFile"))
		assert.Empty(t, s.libraryFiles())
	})

	t.Run("audio Bot API cannot serve is retried by resending", func(t *testing.T) {
		s := newScenario(t)
		audio := s.uploadAudio("track.mp3")
		s.botAPI.failGetFile(audio.FileID, alwaysFail)
		first := s.audioMessage(alice, audio)
		s.send(first)
		s.waitIngest()
		s.botAPI.failGetFile(audio.FileID, 0)
		second := s.audioMessage(alice, audio)

		s.send(second)
		s.waitIngest()

		assert.Equal(t, []string{"👀", "👎"}, s.botAPI.reactionsOn(t, first.Message.ID))
		assert.Equal(t, []string{"👀", "👍"}, s.botAPI.reactionsOn(t, second.Message.ID))
		assert.Equal(t, []string{fixtureTrackPath}, s.personalFiles(alice))
	})

	t.Run("stranger's audio is ignored", func(t *testing.T) {
		s := newScenario(t)
		audio := s.uploadAudio("track.mp3")

		s.send(s.audioMessage(stranger, audio))
		s.waitIngest()

		assert.Empty(t, s.botAPI.allCalls())
		assert.Empty(t, s.libraryFiles())
	})
}
