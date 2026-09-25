package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetadata(t *testing.T) {
	t.Parallel()

	t.Run("untagged MP3 is laid out by Telegram data", func(t *testing.T) {
		s := newScenario(t)
		audio := s.uploadAudioFile(makeAudio(t, "untagged.mp3", audioSpec{}))
		audio.Performer = "Telegram Artist"
		audio.Title = "Telegram Song"

		s.send(s.audioMessage(alice, audio))
		s.waitIngest()

		assert.Equal(t, []string{"👀", "👍"}, s.botAPI.reactions(t))
		assert.Equal(t, []string{"Telegram Artist/Singles/Telegram Song.mp3"}, s.personalFiles(alice))
		tags := readTags(t, s.personalPath(alice, "Telegram Artist/Singles/Telegram Song.mp3"))
		assert.Equal(t, "Telegram Artist", tags["ARTIST"])
		assert.Equal(t, "Telegram Song", tags["TITLE"])
	})

	t.Run("untagged file is laid out by its name", func(t *testing.T) {
		s := newScenario(t)
		audio := s.uploadAudioFile(makeAudio(t, "Name Artist - Name Song.mp3", audioSpec{}))

		s.send(s.audioMessage(alice, audio))
		s.waitIngest()

		assert.Equal(t, []string{"Name Artist/Singles/Name Song.mp3"}, s.personalFiles(alice))
		tags := readTags(t, s.personalPath(alice, "Name Artist/Singles/Name Song.mp3"))
		assert.Equal(t, "Name Artist", tags["ARTIST"])
		assert.Equal(t, "Name Song", tags["TITLE"])
	})

	t.Run("unrecognized track goes to Inbox", func(t *testing.T) {
		s := newScenario(t)
		msg := s.audioMessage(alice, s.uploadAudioFile(makeAudio(t, "audio_123.mp3", audioSpec{})))

		s.send(msg)
		s.waitIngest()

		assert.Equal(t, []string{"👀", "👍"}, s.botAPI.reactions(t))
		assert.Equal(t, []string{"Inbox/audio_123.mp3"}, s.personalFiles(alice))
		replies := s.botAPI.replies(t)
		require.Len(t, replies, 1)
		assert.Equal(t, msg.Message.ID, replies[0].ReplyTo)
		assert.Contains(t, replies[0].Text, "Inbox")
	})

	t.Run("file tags beat Telegram data", func(t *testing.T) {
		s := newScenario(t)
		audio := s.uploadAudio("track.mp3")
		audio.Performer = "Telegram Artist"
		audio.Title = "Telegram Song"

		s.send(s.audioMessage(alice, audio))
		s.waitIngest()

		assert.Equal(t, []string{fixtureTrackPath}, s.personalFiles(alice))
	})

	t.Run("Telegram data fills fields tags lack", func(t *testing.T) {
		s := newScenario(t)
		audio := s.uploadAudioFile(makeAudio(t, "partial.mp3", audioSpec{
			Tags: map[string]string{"title": "Tagged Title", "album": "Tagged Album"},
		}))
		audio.Performer = "Telegram Artist"
		audio.Title = "Telegram Song"

		s.send(s.audioMessage(alice, audio))
		s.waitIngest()

		assert.Equal(t, []string{"Telegram Artist/Tagged Album/Tagged Title.mp3"}, s.personalFiles(alice))
	})
}
