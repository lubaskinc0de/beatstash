package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/beatstash/e2e/harness"
	"github.com/lubaskinc0de/beatstash/e2e/harness/audiofile"
	"github.com/lubaskinc0de/beatstash/e2e/harness/telegram"
)

func TestTrackFileVersion(t *testing.T) {
	t.Parallel()

	t.Run("feed sends the FLAC that replaced the MP3", func(t *testing.T) {
		s := harness.New(t)
		s.Uploaded(alice, s.UploadAudioFile(audiofile.Generate(t, "song.mp3", audiofile.Spec{Bitrate: "128k", Tags: audiofile.SongTags})))
		flac := s.UploadDocument(audiofile.Generate(t, "song.flac", audiofile.Spec{Tags: audiofile.SongTags}), "audio/flac")
		replaced := s.DocumentMessage(alice, flac)
		s.Send(replaced)
		s.WaitIngest()
		s.ShareTrack(alice, dupSongButton)
		query := s.InlineQuery(bob, "shared")

		s.Send(query)

		assert.Equal(t, []string{flac.FileID}, telegram.DocumentFileIDs(s.Telegram.InlineAnswerTo(t, query)))
	})

	t.Run("worse Duplicate sent does not become the Track's file", func(t *testing.T) {
		s := harness.New(t)
		flac := s.UploadDocument(audiofile.Generate(t, "song.flac", audiofile.Spec{Tags: audiofile.SongTags}), "audio/flac")
		stored := s.DocumentMessage(alice, flac)
		s.Send(stored)
		s.WaitIngest()
		s.Uploaded(alice, s.UploadAudioFile(audiofile.Generate(t, "song.mp3", audiofile.Spec{Bitrate: "320k", Tags: audiofile.SongTags})))
		s.ShareTrack(alice, dupSongButton)
		query := s.InlineQuery(bob, "shared")

		s.Send(query)

		answer := s.Telegram.InlineAnswerTo(t, query)
		assert.Equal(t, []string{flac.FileID}, telegram.DocumentFileIDs(answer))
		assert.Empty(t, telegram.AudioFileIDs(answer))
	})

	t.Run("track replaced from Zvuk goes to the storage chat again", func(t *testing.T) {
		s := harness.New(t, harness.WithStorageChat(storageChat))
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukSong("101", "Only Song", false)
		s.LikeOnZvuk("101")
		s.ImportZvuk(alice)
		s.WaitIngest()
		s.AddZvukSong("102", "Only Song", true)
		s.LikeOnZvuk("101", "102")

		s.ImportZvuk(alice)
		s.WaitIngest()

		assert.Equal(t, []string{"Zvuk Band/Zvuk Album (2021)/01 - Only Song.flac"}, s.PersonalFiles(alice))
		sent := s.Telegram.CallsTo("sendAudio")
		require.Len(t, sent, 2)
		assert.Equal(t, "Only Song.flac", sent[1].Params[telegram.UploadedFileParam])
	})
}
