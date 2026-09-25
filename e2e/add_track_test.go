package e2e

import (
	"testing"

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
