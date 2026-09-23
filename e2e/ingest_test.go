package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIngest(t *testing.T) {
	t.Run("allowed user's audio lands in Library", func(t *testing.T) {
		s := newScenario(t)
		audio := s.uploadAudio("track.mp3")

		s.send(s.audioMessage(allowedUser, audio))

		assert.Equal(t, []string{"👀", "👍"}, s.botAPI.Reactions(t))
		assert.Equal(t, []string{checksum(t, fixturePath("track.mp3"))}, s.libraryChecksums())
	})

	t.Run("stranger's audio is ignored", func(t *testing.T) {
		s := newScenario(t)
		audio := s.uploadAudio("track.mp3")

		s.send(s.audioMessage(stranger, audio))

		assert.Empty(t, s.botAPI.Calls())
		assert.Empty(t, s.libraryChecksums())
	})
}
