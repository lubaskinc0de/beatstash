package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/audiofile"
)

func TestDuplicates(t *testing.T) {
	t.Parallel()

	t.Run("same MP3 sent twice is stored once", func(t *testing.T) {
		s := harness.New(t)
		mp3 := audiofile.Generate(t, "song.mp3", audiofile.Spec{Tags: audiofile.SongTags})
		s.Uploaded(alice, s.UploadAudioFile(mp3))
		second := s.AudioMessage(alice, s.UploadAudioFile(mp3))

		s.Send(second)
		s.WaitIngest()

		assert.Equal(t, []string{"Artist/Album/01 - Dup Song.mp3"}, s.PersonalFiles(alice))
		assert.Equal(t, []string{"👀", "👍"}, s.Telegram.ReactionsOn(t, second.Message.ID))
		harness.AssertAlreadyExists(t, s, alice, second.Message.ID)
	})

	t.Run("the same Telegram audio sent again is stored once and gets its reaction", func(t *testing.T) {
		s := harness.New(t)
		audio := s.UploadAudio("track.mp3")
		s.Uploaded(alice, audio)
		resent := s.AudioMessage(alice, audio)

		s.Send(resent)
		s.WaitIngest()

		assert.Equal(t, []string{audiofile.FixtureTrackPath}, s.PersonalFiles(alice))
		assert.Equal(t, []string{"👀", "👍"}, s.Telegram.ReactionsOn(t, resent.Message.ID))
		harness.AssertAlreadyExists(t, s, alice, resent.Message.ID)
	})

	t.Run("FLAC replaces MP3 and inline sends the FLAC", func(t *testing.T) {
		s := harness.New(t)
		mp3 := s.UploadAudioFile(audiofile.Generate(t, "song.mp3", audiofile.Spec{Bitrate: "128k", Tags: audiofile.SongTags}))
		s.Send(s.AudioMessage(alice, mp3))
		s.WaitIngest()
		flac := s.UploadDocument(audiofile.Generate(t, "song.flac", audiofile.Spec{Tags: audiofile.SongTags}), "audio/flac")

		s.Send(s.DocumentMessage(alice, flac))
		s.WaitIngest()

		assert.Equal(t, []string{"Artist/Album/01 - Dup Song.flac"}, s.PersonalFiles(alice))
		assert.Empty(t, s.Telegram.Replies(t))
		account := s.Navidrome.CreateAccount(t, "alice")
		s.Link(alice, account)
		track := s.Navidrome.IndexedTrack(t, account, s.Library, "Dup Song")
		s.Navidrome.StartPlaying(t, account, track.ID)
		query := s.InlineQuery(alice, "np")
		s.Send(query)
		answers := s.Telegram.InlineAnswers(t)
		require.Len(t, answers, 1)
		require.Len(t, answers[0].Results, 1)
		assert.Equal(t, "document", answers[0].Results[0].Type)
		assert.Equal(t, flac.FileID, answers[0].Results[0].DocumentFileID)
	})

	t.Run("higher bitrate MP3 replaces lower", func(t *testing.T) {
		s := harness.New(t)
		low := audiofile.Generate(t, "low.mp3", audiofile.Spec{Bitrate: "128k", Tags: audiofile.SongTags})
		high := audiofile.Generate(t, "high.mp3", audiofile.Spec{Bitrate: "320k", Tags: audiofile.SongTags})
		s.Send(s.AudioMessage(alice, s.UploadAudioFile(low)))
		s.WaitIngest()

		s.Send(s.AudioMessage(alice, s.UploadAudioFile(high)))
		s.WaitIngest()

		assert.Equal(t, []string{"Artist/Album/01 - Dup Song.mp3"}, s.PersonalFiles(alice))
		assert.Equal(t, 320, audiofile.BitrateKbps(t, s.PersonalPath(alice, "Artist/Album/01 - Dup Song.mp3")))
	})

	t.Run("MP3 after FLAC keeps FLAC", func(t *testing.T) {
		s := harness.New(t)
		flac := s.UploadDocument(audiofile.Generate(t, "song.flac", audiofile.Spec{Tags: audiofile.SongTags}), "audio/flac")
		s.Send(s.DocumentMessage(alice, flac))
		s.WaitIngest()
		mp3 := s.AudioMessage(alice, s.UploadAudioFile(audiofile.Generate(t, "song.mp3", audiofile.Spec{Bitrate: "320k", Tags: audiofile.SongTags})))

		s.Send(mp3)
		s.WaitIngest()

		assert.Equal(t, []string{"Artist/Album/01 - Dup Song.flac"}, s.PersonalFiles(alice))
		harness.AssertAlreadyExists(t, s, alice, mp3.Message.ID)
	})

	t.Run("same song from another album is kept", func(t *testing.T) {
		s := harness.New(t)
		compilation := map[string]string{"artist": "Artist", "album": "Best Of", "track": "5", "title": "Dup Song"}
		s.Send(s.AudioMessage(alice, s.UploadAudioFile(audiofile.Generate(t, "a.mp3", audiofile.Spec{Tags: audiofile.SongTags}))))
		s.WaitIngest()

		s.Send(s.AudioMessage(alice, s.UploadAudioFile(audiofile.Generate(t, "b.mp3", audiofile.Spec{Tags: compilation}))))
		s.WaitIngest()

		assert.Equal(t, []string{"Artist/Album/01 - Dup Song.mp3", "Artist/Best Of/05 - Dup Song.mp3"}, s.PersonalFiles(alice))
		assert.Empty(t, s.Telegram.Replies(t))
	})

	t.Run("duration off by more than 2 s is another track", func(t *testing.T) {
		s := harness.New(t)
		s.Send(s.AudioMessage(alice, s.UploadAudioFile(audiofile.Generate(t, "a.mp3", audiofile.Spec{Seconds: 1.5, Tags: audiofile.SongTags}))))
		s.WaitIngest()

		s.Send(s.AudioMessage(alice, s.UploadAudioFile(audiofile.Generate(t, "b.mp3", audiofile.Spec{Seconds: 4.5, Tags: audiofile.SongTags}))))
		s.WaitIngest()

		assert.Len(t, s.PersonalFiles(alice), 2)
		assert.Empty(t, s.Telegram.Replies(t))
	})

	t.Run("duration off by less than 2 s is a duplicate", func(t *testing.T) {
		s := harness.New(t)
		s.Send(s.AudioMessage(alice, s.UploadAudioFile(audiofile.Generate(t, "a.mp3", audiofile.Spec{Seconds: 1.5, Tags: audiofile.SongTags}))))
		s.WaitIngest()
		second := s.AudioMessage(alice, s.UploadAudioFile(audiofile.Generate(t, "b.mp3", audiofile.Spec{Seconds: 3, Tags: audiofile.SongTags})))

		s.Send(second)
		s.WaitIngest()

		assert.Len(t, s.PersonalFiles(alice), 1)
		harness.AssertAlreadyExists(t, s, alice, second.Message.ID)
	})

	t.Run("case and spacing do not make another track", func(t *testing.T) {
		s := harness.New(t)
		s.Send(s.AudioMessage(alice, s.UploadAudioFile(audiofile.Generate(t, "a.mp3", audiofile.Spec{Tags: audiofile.SongTags}))))
		s.WaitIngest()
		shouting := map[string]string{"artist": "ARTIST", "album": "album", "track": "1", "title": " Dup   SONG "}
		second := s.AudioMessage(alice, s.UploadAudioFile(audiofile.Generate(t, "b.mp3", audiofile.Spec{Tags: shouting})))

		s.Send(second)
		s.WaitIngest()

		assert.Equal(t, []string{"Artist/Album/01 - Dup Song.mp3"}, s.PersonalFiles(alice))
		harness.AssertAlreadyExists(t, s, alice, second.Message.ID)
	})
}
