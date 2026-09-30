package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/audiofile"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/ingest"
)

func TestFormats(t *testing.T) {
	t.Parallel()

	t.Run("30 MB FLAC document lands in Library", func(t *testing.T) {
		s := harness.New(t)
		flac := audiofile.Generate(t, "big.flac", audiofile.Spec{
			Seconds: 240,
			Noise:   true,
			Tags:    map[string]string{"artist": "Noise", "album": "Static", "title": "Hiss", "track": "3"},
		})
		info, err := os.Stat(flac)
		require.NoError(t, err)
		require.Greater(t, info.Size(), int64(30<<20))
		doc := s.UploadDocument(flac, "audio/flac")

		s.Send(s.DocumentMessage(alice, doc))
		s.WaitIngest()

		assert.Equal(t, []string{"👀", "👍"}, s.Telegram.Reactions(t))
		assert.Equal(t, []string{"Noise/Static/03 - Hiss.flac"}, s.PersonalFiles(alice))
	})

	t.Run("each supported format lands in Library", func(t *testing.T) {
		for _, name := range []string{"track.mp3", "track.flac", "track.m4a", "track.ogg", "track.opus"} {
			t.Run(name, func(t *testing.T) {
				s := harness.New(t)
				doc := s.UploadDocument(audiofile.Fixture(name), "")

				s.Send(s.DocumentMessage(alice, doc))
				s.WaitIngest()

				assert.Equal(t, []string{"👀", "👍"}, s.Telegram.Reactions(t))
				expected := strings.TrimSuffix(audiofile.FixtureTrackPath, ".mp3") + filepath.Ext(name)
				assert.Equal(t, []string{expected}, s.PersonalFiles(alice))
			})
		}
	})

	t.Run("PDF is rejected as unsupported", func(t *testing.T) {
		s := harness.New(t)
		pdf := audiofile.WriteFile(t, "scan.pdf", []byte("%PDF-1.4 not music"))
		msg := s.DocumentMessage(alice, s.UploadDocument(pdf, "application/pdf"))

		s.Send(msg)
		s.WaitIngest()

		assert.Equal(t, []string{"👎"}, s.Telegram.Reactions(t))
		replies := s.Telegram.Replies(t)
		require.Len(t, replies, 1)
		assert.Equal(t, msg.Message.ID, replies[0].ReplyTo)
		assert.Contains(t, replies[0].Text, s.Catalog(alice).UploadFailed(ingest.ReasonUnsupportedFormat))
		assert.Empty(t, s.LibraryFiles())
	})

	t.Run("voice message is ignored", func(t *testing.T) {
		s := harness.New(t)
		fileID, _ := s.Upload(audiofile.Fixture("track.ogg"))

		s.Send(s.Message(alice, func(m *models.Message) {
			m.Voice = &models.Voice{FileID: fileID, FileUniqueID: fileID + "-unique", Duration: 1}
		}))
		s.WaitIngest()

		assert.Empty(t, s.Telegram.AllCalls())
		assert.Empty(t, s.LibraryFiles())
	})

	t.Run("video note is ignored", func(t *testing.T) {
		s := harness.New(t)

		s.Send(s.Message(alice, func(m *models.Message) {
			m.VideoNote = &models.VideoNote{FileID: "note", FileUniqueID: "note-unique", Duration: 1}
		}))
		s.WaitIngest()

		assert.Empty(t, s.Telegram.AllCalls())
		assert.Empty(t, s.LibraryFiles())
	})

	t.Run("Bot API working file is removed after Ingest", func(t *testing.T) {
		s := harness.New(t)
		audio := s.UploadAudio("track.mp3")
		working := s.Telegram.PathOf(audio.FileID)

		s.Send(s.AudioMessage(alice, audio))
		s.WaitIngest()

		assert.Equal(t, []string{"👀", "👍"}, s.Telegram.Reactions(t))
		assert.NoFileExists(t, working)
	})

	t.Run("Bot API working file of an Ingest cut by a restart is removed", func(t *testing.T) {
		s := harness.New(t)
		audio := s.UploadAudio("track.mp3")
		working := s.Telegram.PathOf(audio.FileID)
		held := s.Telegram.HoldGetFile()
		s.Send(s.AudioMessage(alice, audio))
		<-held

		s.Restart()
		s.Telegram.ReleaseGetFile()
		s.WaitIngest()

		assert.NoFileExists(t, working)
	})
}

func TestWavToFlac(t *testing.T) {
	t.Parallel()

	t.Run("WAV lands as lossless FLAC", func(t *testing.T) {
		s := harness.New(t)
		wav := s.UploadAudioFile(audiofile.Generate(t, "take.wav", audiofile.Spec{Seconds: 2}))
		wav.Performer = "Wave Artist"
		wav.Title = "Wave Song"

		s.Send(s.AudioMessage(alice, wav))
		s.WaitIngest()

		assert.Equal(t, []string{"👀", "👍"}, s.Telegram.Reactions(t))
		assert.Equal(t, []string{"Wave Artist/Singles/Wave Song.flac"}, s.PersonalFiles(alice))
		path := s.PersonalPath(alice, "Wave Artist/Singles/Wave Song.flac")
		codec, seconds := audiofile.Codec(t, path)
		assert.Equal(t, "flac", codec)
		assert.InDelta(t, 2.0, seconds, 0.05)
		tags := audiofile.Tags(t, path)
		assert.Equal(t, "Wave Artist", tags["ARTIST"])
		assert.Equal(t, "Wave Song", tags["TITLE"])
	})

	t.Run("WAV document lands as FLAC", func(t *testing.T) {
		s := harness.New(t)
		wav := audiofile.Generate(t, "Doc Artist - Doc Song.wav", audiofile.Spec{})

		s.Send(s.DocumentMessage(alice, s.UploadDocument(wav, "audio/x-wav")))
		s.WaitIngest()

		assert.Equal(t, []string{"Doc Artist/Singles/Doc Song.flac"}, s.PersonalFiles(alice))
	})
}
