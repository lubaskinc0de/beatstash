package e2e

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/audiofile"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/telegram"
)

func TestIngest(t *testing.T) {
	t.Parallel()

	t.Run("tagged MP3 lands at artist/album/track path", func(t *testing.T) {
		s := harness.New(t)
		audio := s.UploadAudio("track.mp3")

		s.Send(s.AudioMessage(alice, audio))
		s.WaitIngest()

		assert.Equal(t, []string{"👀", "👍"}, s.Telegram.Reactions(t))
		assert.Equal(t, []string{audiofile.FixtureTrackPath}, s.PersonalFiles(alice))
	})

	t.Run("audio is acknowledged before Ingest", func(t *testing.T) {
		s := harness.New(t, harness.WithoutWorkers())
		audio := s.UploadAudio("track.mp3")

		s.Send(s.AudioMessage(alice, audio))

		assert.Equal(t, []string{"👀"}, s.Telegram.Reactions(t))
		assert.Empty(t, s.Telegram.CallsTo("getFile"))
		assert.Empty(t, s.LibraryFiles())
	})

	t.Run("audio Bot API cannot serve is retried by resending", func(t *testing.T) {
		s := harness.New(t)
		audio := s.UploadAudio("track.mp3")
		s.Telegram.FailGetFile(audio.FileID, telegram.AlwaysFail)
		first := s.AudioMessage(alice, audio)
		s.Send(first)
		s.WaitIngest()
		s.Telegram.FailGetFile(audio.FileID, 0)
		second := s.AudioMessage(alice, audio)

		s.Send(second)
		s.WaitIngest()

		assert.Equal(t, []string{"👀", "👎"}, s.Telegram.ReactionsOn(t, first.Message.ID))
		assert.Equal(t, []string{"👀", "👍"}, s.Telegram.ReactionsOn(t, second.Message.ID))
		assert.Equal(t, []string{audiofile.FixtureTrackPath}, s.PersonalFiles(alice))
	})

	t.Run("stranger's audio is ignored", func(t *testing.T) {
		s := harness.New(t)
		audio := s.UploadAudio("track.mp3")

		s.Send(s.AudioMessage(stranger, audio))
		s.WaitIngest()

		assert.Empty(t, s.Telegram.AllCalls())
		assert.Empty(t, s.LibraryFiles())
	})
}

func TestIngestAnswer(t *testing.T) {
	t.Parallel()

	t.Run("two instances answer a track once", func(t *testing.T) {
		s := harness.New(t, harness.WithLeaseTTL(time.Minute))
		s.StartReplica()
		msg := s.AudioMessage(alice, s.UploadAudio("track.mp3"))
		s.Send(msg)
		answer := s.Telegram.Hold("setMessageReaction")
		<-answer.Arrived()

		s.PollAny()
		assert.Len(t, s.Telegram.CallsTo("setMessageReaction"), 1, "another instance acted meanwhile")
		answer.Release()
		s.WaitIngest()

		assert.Equal(t, []string{"👀", "👍"}, s.Telegram.ReactionsOn(t, msg.Message.ID))
	})

	t.Run("answer Telegram failed to take comes again", func(t *testing.T) {
		s := harness.New(t)
		msg := s.AudioMessage(alice, s.UploadAudio("track.mp3"))
		s.Send(msg)
		s.Telegram.FailCalls("setMessageReaction", 1, http.StatusInternalServerError)

		s.WaitIngest()

		assert.Equal(t, []string{"👀", "👍"}, s.Telegram.ReactionsOn(t, msg.Message.ID))
	})

	t.Run("answer to a message Telegram no longer has is dropped", func(t *testing.T) {
		s := harness.New(t)
		msg := s.AudioMessage(alice, s.UploadAudio("track.mp3"))
		s.Send(msg)
		s.Telegram.FailCalls("setMessageReaction", 1, http.StatusBadRequest)

		s.WaitIngest()

		assert.Equal(t, []string{"👀"}, s.Telegram.ReactionsOn(t, msg.Message.ID))
	})

	t.Run("answer cut by a crash comes after it", func(t *testing.T) {
		s := harness.New(t)
		msg := s.AudioMessage(alice, s.UploadAudio("track.mp3"))
		s.Send(msg)
		answer := s.Telegram.Hold("setMessageReaction")
		<-answer.Arrived()

		s.Restart()
		s.WaitIngest()

		assert.Equal(t, []string{"👀", "👍"}, s.Telegram.ReactionsOn(t, msg.Message.ID))
	})
}
