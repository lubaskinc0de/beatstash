package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var songTags = map[string]string{"artist": "Artist", "album": "Album", "track": "1", "title": "Dup Song"}

func TestDuplicates(t *testing.T) {
	t.Parallel()

	t.Run("same MP3 sent twice is stored once", func(t *testing.T) {
		s := newScenario(t)
		mp3 := makeAudio(t, "song.mp3", audioSpec{Tags: songTags})
		s.send(s.audioMessage(alice, s.uploadAudioFile(mp3)))
		s.waitIngest()
		second := s.audioMessage(alice, s.uploadAudioFile(mp3))

		s.send(second)
		s.waitIngest()

		assert.Equal(t, []string{"Artist/Album/01 - Dup Song.mp3"}, s.personalFiles(alice))
		assert.Equal(t, []string{"👀", "👍"}, s.botAPI.reactionsOn(t, second.Message.ID))
		assertAlreadyExists(t, s, second.Message.ID)
	})

	t.Run("forwarded audio is stored once", func(t *testing.T) {
		s := newScenario(t)
		audio := s.uploadAudio("track.mp3")
		s.send(s.audioMessage(alice, audio))
		s.waitIngest()
		forward := s.audioMessage(alice, audio)

		s.send(forward)
		s.waitIngest()

		assert.Equal(t, []string{fixtureTrackPath}, s.personalFiles(alice))
		assertAlreadyExists(t, s, forward.Message.ID)
	})

	t.Run("FLAC replaces MP3 and inline still sends the track", func(t *testing.T) {
		s := newScenario(t)
		mp3 := s.uploadAudioFile(makeAudio(t, "song.mp3", audioSpec{Bitrate: "128k", Tags: songTags}))
		s.send(s.audioMessage(alice, mp3))
		s.waitIngest()
		flac := s.uploadDocument(makeAudio(t, "song.flac", audioSpec{Tags: songTags}), "audio/flac")

		s.send(s.documentMessage(alice, flac))
		s.waitIngest()

		assert.Equal(t, []string{"Artist/Album/01 - Dup Song.flac"}, s.personalFiles(alice))
		assert.Empty(t, s.botAPI.replies(t))
		account := env.navidrome.createAccount(t, "alice")
		s.link(alice, account)
		track := env.navidrome.indexedTrack(t, account, s.library, "Dup Song")
		env.navidrome.startPlaying(t, account, track.ID)
		query := s.inlineQuery(alice, "np")
		s.send(query)
		answers := s.botAPI.inlineAnswers(t)
		require.Len(t, answers, 1)
		require.Len(t, answers[0].Results, 1)
		assert.Equal(t, "audio", answers[0].Results[0].Type)
		assert.Equal(t, mp3.FileID, answers[0].Results[0].AudioFileID)
	})

	t.Run("higher bitrate MP3 replaces lower", func(t *testing.T) {
		s := newScenario(t)
		low := makeAudio(t, "low.mp3", audioSpec{Bitrate: "128k", Tags: songTags})
		high := makeAudio(t, "high.mp3", audioSpec{Bitrate: "320k", Tags: songTags})
		s.send(s.audioMessage(alice, s.uploadAudioFile(low)))
		s.waitIngest()

		s.send(s.audioMessage(alice, s.uploadAudioFile(high)))
		s.waitIngest()

		assert.Equal(t, []string{"Artist/Album/01 - Dup Song.mp3"}, s.personalFiles(alice))
		assert.Equal(t, 320, bitrateKbps(t, s.personalPath(alice, "Artist/Album/01 - Dup Song.mp3")))
	})

	t.Run("MP3 after FLAC keeps FLAC", func(t *testing.T) {
		s := newScenario(t)
		flac := s.uploadDocument(makeAudio(t, "song.flac", audioSpec{Tags: songTags}), "audio/flac")
		s.send(s.documentMessage(alice, flac))
		s.waitIngest()
		mp3 := s.audioMessage(alice, s.uploadAudioFile(makeAudio(t, "song.mp3", audioSpec{Bitrate: "320k", Tags: songTags})))

		s.send(mp3)
		s.waitIngest()

		assert.Equal(t, []string{"Artist/Album/01 - Dup Song.flac"}, s.personalFiles(alice))
		assertAlreadyExists(t, s, mp3.Message.ID)
	})

	t.Run("same song from another album is kept", func(t *testing.T) {
		s := newScenario(t)
		compilation := map[string]string{"artist": "Artist", "album": "Best Of", "track": "5", "title": "Dup Song"}
		s.send(s.audioMessage(alice, s.uploadAudioFile(makeAudio(t, "a.mp3", audioSpec{Tags: songTags}))))
		s.waitIngest()

		s.send(s.audioMessage(alice, s.uploadAudioFile(makeAudio(t, "b.mp3", audioSpec{Tags: compilation}))))
		s.waitIngest()

		assert.Equal(t, []string{"Artist/Album/01 - Dup Song.mp3", "Artist/Best Of/05 - Dup Song.mp3"}, s.personalFiles(alice))
		assert.Empty(t, s.botAPI.replies(t))
	})

	t.Run("duration off by more than 2 s is another track", func(t *testing.T) {
		s := newScenario(t)
		s.send(s.audioMessage(alice, s.uploadAudioFile(makeAudio(t, "a.mp3", audioSpec{Seconds: 1.5, Tags: songTags}))))
		s.waitIngest()

		s.send(s.audioMessage(alice, s.uploadAudioFile(makeAudio(t, "b.mp3", audioSpec{Seconds: 4.5, Tags: songTags}))))
		s.waitIngest()

		assert.Len(t, s.personalFiles(alice), 2)
		assert.Empty(t, s.botAPI.replies(t))
	})

	t.Run("duration off by less than 2 s is a duplicate", func(t *testing.T) {
		s := newScenario(t)
		s.send(s.audioMessage(alice, s.uploadAudioFile(makeAudio(t, "a.mp3", audioSpec{Seconds: 1.5, Tags: songTags}))))
		s.waitIngest()
		second := s.audioMessage(alice, s.uploadAudioFile(makeAudio(t, "b.mp3", audioSpec{Seconds: 3, Tags: songTags})))

		s.send(second)
		s.waitIngest()

		assert.Len(t, s.personalFiles(alice), 1)
		assertAlreadyExists(t, s, second.Message.ID)
	})

	t.Run("case and spacing do not make another track", func(t *testing.T) {
		s := newScenario(t)
		s.send(s.audioMessage(alice, s.uploadAudioFile(makeAudio(t, "a.mp3", audioSpec{Tags: songTags}))))
		s.waitIngest()
		shouting := map[string]string{"artist": "ARTIST", "album": "album", "track": "1", "title": " Dup   SONG "}
		second := s.audioMessage(alice, s.uploadAudioFile(makeAudio(t, "b.mp3", audioSpec{Tags: shouting})))

		s.send(second)
		s.waitIngest()

		assert.Equal(t, []string{"Artist/Album/01 - Dup Song.mp3"}, s.personalFiles(alice))
		assertAlreadyExists(t, s, second.Message.ID)
	})
}

func assertAlreadyExists(t *testing.T, s *scenario, messageID int) {
	t.Helper()

	replies := s.botAPI.replies(t)
	require.Len(t, replies, 1)
	assert.Equal(t, messageID, replies[0].ReplyTo)
	assert.Contains(t, replies[0].Text, "уже есть")
}
