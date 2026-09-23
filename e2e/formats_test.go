package e2e

import (
	"os"
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormats(t *testing.T) {
	t.Run("30 MB FLAC document lands in Library", func(t *testing.T) {
		s := newScenario(t)
		flac := makeAudio(t, "big.flac", audioSpec{
			Seconds: 240,
			Noise:   true,
			Tags:    map[string]string{"artist": "Noise", "album": "Static", "title": "Hiss", "track": "3"},
		})
		info, err := os.Stat(flac)
		require.NoError(t, err)
		require.Greater(t, info.Size(), int64(30<<20))
		doc := s.uploadDocument(flac, "audio/flac")

		s.send(s.documentMessage(alice, doc))
		s.waitIngest()

		assert.Equal(t, []string{"👀", "👍"}, s.botAPI.Reactions(t))
		assert.Equal(t, []string{"Noise/Static/03 - Hiss.flac"}, s.libraryFiles())
	})

	t.Run("each supported format lands in Library", func(t *testing.T) {
		for _, fixture := range []string{"track.mp3", "track.flac", "track.m4a", "track.ogg", "track.opus"} {
			t.Run(fixture, func(t *testing.T) {
				s := newScenario(t)
				doc := s.uploadDocument(fixturePath(fixture), "")

				s.send(s.documentMessage(alice, doc))
				s.waitIngest()

				assert.Equal(t, []string{"👀", "👍"}, s.botAPI.Reactions(t))
				assert.Len(t, s.libraryFiles(), 1)
			})
		}
	})

	t.Run("PDF is rejected as unsupported", func(t *testing.T) {
		s := newScenario(t)
		pdf := writeFile(t, "scan.pdf", []byte("%PDF-1.4 not music"))
		msg := s.documentMessage(alice, s.uploadDocument(pdf, "application/pdf"))

		s.send(msg)
		s.waitIngest()

		assert.Equal(t, []string{"👎"}, s.botAPI.Reactions(t))
		replies := s.botAPI.Replies(t)
		require.Len(t, replies, 1)
		assert.Equal(t, msg.Message.ID, replies[0].ReplyTo)
		assert.Contains(t, replies[0].Text, "формат не поддерживается")
		assert.Empty(t, s.libraryFiles())
	})

	t.Run("voice message is ignored", func(t *testing.T) {
		s := newScenario(t)
		fileID, _ := s.upload(fixturePath("track.ogg"))

		s.send(s.message(alice, func(m *models.Message) {
			m.Voice = &models.Voice{FileID: fileID, FileUniqueID: fileID + "-unique", Duration: 1}
		}))
		s.waitIngest()

		assert.Empty(t, s.botAPI.Calls())
		assert.Empty(t, s.libraryFiles())
	})

	t.Run("video note is ignored", func(t *testing.T) {
		s := newScenario(t)

		s.send(s.message(alice, func(m *models.Message) {
			m.VideoNote = &models.VideoNote{FileID: "note", FileUniqueID: "note-unique", Duration: 1}
		}))
		s.waitIngest()

		assert.Empty(t, s.botAPI.Calls())
		assert.Empty(t, s.libraryFiles())
	})

	t.Run("Bot API working file is removed after Ingest", func(t *testing.T) {
		s := newScenario(t)
		audio := s.uploadAudio("track.mp3")
		working := s.botAPI.pathOf(audio.FileID)

		s.send(s.audioMessage(alice, audio))
		s.waitIngest()

		assert.Equal(t, []string{"👀", "👍"}, s.botAPI.Reactions(t))
		assert.NoFileExists(t, working)
	})
}

func TestWavToFlac(t *testing.T) {
	t.Run("WAV lands as lossless FLAC", func(t *testing.T) {
		s := newScenario(t)
		wav := s.uploadAudioFile(makeAudio(t, "take.wav", audioSpec{Seconds: 2}))
		wav.Performer = "Wave Artist"
		wav.Title = "Wave Song"

		s.send(s.audioMessage(alice, wav))
		s.waitIngest()

		assert.Equal(t, []string{"👀", "👍"}, s.botAPI.Reactions(t))
		assert.Equal(t, []string{"Wave Artist/Singles/Wave Song.flac"}, s.libraryFiles())
		path := s.libraryPath("Wave Artist/Singles/Wave Song.flac")
		codec, seconds := audioCodec(t, path)
		assert.Equal(t, "flac", codec)
		assert.InDelta(t, 2.0, seconds, 0.05)
		tags := readTags(t, path)
		assert.Equal(t, "Wave Artist", tags["ARTIST"])
		assert.Equal(t, "Wave Song", tags["TITLE"])
	})

	t.Run("WAV document lands as FLAC", func(t *testing.T) {
		s := newScenario(t)
		wav := makeAudio(t, "Doc Artist - Doc Song.wav", audioSpec{})

		s.send(s.documentMessage(alice, s.uploadDocument(wav, "audio/x-wav")))
		s.waitIngest()

		assert.Equal(t, []string{"Doc Artist/Singles/Doc Song.flac"}, s.libraryFiles())
	})
}
