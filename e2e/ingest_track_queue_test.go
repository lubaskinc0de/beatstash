package e2e

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/audiofile"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/telegram"
)

func TestQueue(t *testing.T) {
	t.Parallel()

	t.Run("transient Bot API failures are retried", func(t *testing.T) {
		s := harness.New(t)
		audio := s.UploadAudio("track.mp3")
		s.Telegram.FailGetFile(audio.FileID, 2)

		s.Send(s.AudioMessage(alice, audio))
		s.WaitIngest()

		assert.Equal(t, []string{"👀", "👍"}, s.Telegram.Reactions(t))
		assert.Equal(t, []string{audiofile.FixtureTrackPath}, s.PersonalFiles(alice))
		assert.Empty(t, s.Telegram.Replies(t))
	})

	t.Run("persistent Bot API failure gives up after retries", func(t *testing.T) {
		s := harness.New(t)
		audio := s.UploadAudio("track.mp3")
		s.Telegram.FailGetFile(audio.FileID, telegram.AlwaysFail)
		msg := s.AudioMessage(alice, audio)

		s.Send(msg)
		s.WaitIngest()

		assert.Len(t, s.Telegram.CallsTo("getFile"), 4)
		assert.Equal(t, []string{"👀", "👎"}, s.Telegram.Reactions(t))
		replies := s.Telegram.Replies(t)
		require.Len(t, replies, 1)
		assert.Equal(t, msg.Message.ID, replies[0].ReplyTo)
		assert.Contains(t, replies[0].Text, "Telegram не отдал файл")
		assert.Empty(t, s.LibraryFiles())
	})

	t.Run("corrupt file fails at once", func(t *testing.T) {
		s := harness.New(t)
		garbage := audiofile.WriteFile(t, "broken.mp3", []byte(strings.Repeat("not an mp3 at all ", 1000)))
		msg := s.AudioMessage(alice, s.UploadAudioFile(garbage))

		s.Send(msg)
		s.WaitIngest()

		assert.Len(t, s.Telegram.CallsTo("getFile"), 1)
		assert.Equal(t, []string{"👀", "👎"}, s.Telegram.Reactions(t))
		replies := s.Telegram.Replies(t)
		require.Len(t, replies, 1)
		assert.Contains(t, replies[0].Text, "файл повреждён")
		assert.Empty(t, s.LibraryFiles())
	})

	t.Run("queued tracks survive restart", func(t *testing.T) {
		s := harness.New(t, harness.WithoutWorkers())
		for i := 1; i <= 5; i++ {
			s.Send(s.AudioMessage(alice, s.UploadAudioFile(numberedTrack(t, i))))
		}

		s.Restart()
		s.WaitIngest()

		assert.Len(t, s.PersonalFiles(alice), 5)
	})

	t.Run("track queued before restart gets its reaction after it", func(t *testing.T) {
		s := harness.New(t, harness.WithoutWorkers())
		msg := s.AudioMessage(alice, s.UploadAudio("track.mp3"))
		s.Send(msg)

		s.Restart()
		s.WaitIngest()

		assert.Equal(t, []string{audiofile.FixtureTrackPath}, s.PersonalFiles(alice))
		assert.Equal(t, []string{"👀", "👍"}, s.Telegram.ReactionsOn(t, msg.Message.ID))
	})

	t.Run("tracks in progress survive restart", func(t *testing.T) {
		s := harness.New(t)
		held := s.Telegram.HoldGetFile()
		var msgs []*models.Update
		for i := 1; i <= 5; i++ {
			msg := s.AudioMessage(alice, s.UploadAudioFile(numberedTrack(t, i)))
			msgs = append(msgs, msg)
			s.Send(msg)
		}
		<-held

		s.Restart()
		s.Telegram.ReleaseGetFile()
		s.WaitIngest()

		assert.Len(t, s.PersonalFiles(alice), 5)
		for _, msg := range msgs {
			assert.Equal(t, []string{"👀", "👍"}, s.Telegram.ReactionsOn(t, msg.Message.ID))
		}
	})

	t.Run("batch of 10 tracks is ingested once each", func(t *testing.T) {
		s := harness.New(t)
		var msgs []*models.Update
		for i := 1; i <= 10; i++ {
			msgs = append(msgs, s.AudioMessage(alice, s.UploadAudioFile(numberedTrack(t, i))))
		}

		for _, msg := range msgs {
			s.Send(msg)
		}
		s.WaitIngest()

		assert.Len(t, s.PersonalFiles(alice), 10)
		for _, msg := range msgs {
			assert.Equal(t, []string{"👀", "👍"}, s.Telegram.ReactionsOn(t, msg.Message.ID))
		}
		assert.Empty(t, s.Telegram.Replies(t))
	})
}

func numberedTrack(t *testing.T, n int) string {
	return audiofile.Generate(t, fmt.Sprintf("song-%d.mp3", n), audiofile.Spec{Tags: map[string]string{
		"artist": "Batch", "album": "Many", "track": fmt.Sprint(n), "title": fmt.Sprintf("Song %d", n),
	}})
}
