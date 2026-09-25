package e2e

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueue(t *testing.T) {
	t.Parallel()

	t.Run("transient Bot API failures are retried", func(t *testing.T) {
		s := newScenario(t)
		audio := s.uploadAudio("track.mp3")
		s.botAPI.failGetFile(audio.FileID, 2)

		s.send(s.audioMessage(alice, audio))
		s.waitIngest()

		assert.Equal(t, []string{"👀", "👍"}, s.botAPI.reactions(t))
		assert.Equal(t, []string{fixtureTrackPath}, s.personalFiles(alice))
		assert.Empty(t, s.botAPI.replies(t))
	})

	t.Run("persistent Bot API failure gives up after retries", func(t *testing.T) {
		s := newScenario(t)
		audio := s.uploadAudio("track.mp3")
		s.botAPI.failGetFile(audio.FileID, alwaysFail)
		msg := s.audioMessage(alice, audio)

		s.send(msg)
		s.waitIngest()

		assert.Len(t, s.botAPI.callsTo("getFile"), 4)
		assert.Equal(t, []string{"👀", "👎"}, s.botAPI.reactions(t))
		replies := s.botAPI.replies(t)
		require.Len(t, replies, 1)
		assert.Equal(t, msg.Message.ID, replies[0].ReplyTo)
		assert.Contains(t, replies[0].Text, "Telegram не отдал файл")
		assert.Empty(t, s.libraryFiles())
	})

	t.Run("corrupt file fails at once", func(t *testing.T) {
		s := newScenario(t)
		garbage := writeFile(t, "broken.mp3", []byte(strings.Repeat("not an mp3 at all ", 1000)))
		msg := s.audioMessage(alice, s.uploadAudioFile(garbage))

		s.send(msg)
		s.waitIngest()

		assert.Len(t, s.botAPI.callsTo("getFile"), 1)
		assert.Equal(t, []string{"👀", "👎"}, s.botAPI.reactions(t))
		replies := s.botAPI.replies(t)
		require.Len(t, replies, 1)
		assert.Contains(t, replies[0].Text, "файл повреждён")
		assert.Empty(t, s.libraryFiles())
	})

	t.Run("queued tracks survive restart", func(t *testing.T) {
		s := newScenario(t, withoutWorkers())
		for i := 1; i <= 5; i++ {
			s.send(s.audioMessage(alice, s.uploadAudioFile(numberedTrack(t, i))))
		}

		s.restart()
		s.waitIngest()

		assert.Len(t, s.personalFiles(alice), 5)
	})

	t.Run("tracks in progress survive restart", func(t *testing.T) {
		s := newScenario(t)
		held := s.botAPI.holdGetFile()
		var msgs []*models.Update
		for i := 1; i <= 5; i++ {
			msg := s.audioMessage(alice, s.uploadAudioFile(numberedTrack(t, i)))
			msgs = append(msgs, msg)
			s.send(msg)
		}
		<-held

		s.restart()
		s.botAPI.releaseGetFile()
		s.waitIngest()

		assert.Len(t, s.personalFiles(alice), 5)
		for _, msg := range msgs {
			assert.Equal(t, []string{"👀", "👍"}, s.botAPI.reactionsOn(t, msg.Message.ID))
		}
	})

	t.Run("batch of 10 tracks is ingested once each", func(t *testing.T) {
		s := newScenario(t)
		var msgs []*models.Update
		for i := 1; i <= 10; i++ {
			msgs = append(msgs, s.audioMessage(alice, s.uploadAudioFile(numberedTrack(t, i))))
		}

		for _, msg := range msgs {
			s.send(msg)
		}
		s.waitIngest()

		assert.Len(t, s.personalFiles(alice), 10)
		for _, msg := range msgs {
			assert.Equal(t, []string{"👀", "👍"}, s.botAPI.reactionsOn(t, msg.Message.ID))
		}
		assert.Empty(t, s.botAPI.replies(t))
	})
}

func numberedTrack(t *testing.T, n int) string {
	return makeAudio(t, fmt.Sprintf("song-%d.mp3", n), audioSpec{Tags: map[string]string{
		"artist": "Batch", "album": "Many", "track": fmt.Sprint(n), "title": fmt.Sprintf("Song %d", n),
	}})
}
