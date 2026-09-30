package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/audiofile"
)

func TestPersonalLibrary(t *testing.T) {
	t.Parallel()

	t.Run("upload stays private to its uploader", func(t *testing.T) {
		s := harness.New(t)
		aliceAccount := s.LinkNewAccount(alice)
		bobAccount := s.LinkNewAccount(bob)

		s.Uploaded(alice, s.UploadAudio("track.mp3"))

		assert.Equal(t, []string{audiofile.FixtureTrackPath}, s.PersonalFiles(alice))
		s.Navidrome.IndexedTrack(t, aliceAccount, s.Library, audiofile.FixtureTitle)
		assert.Empty(t, s.Navidrome.SearchFor(t, bobAccount, s.Library, audiofile.FixtureTitle))
	})

	t.Run("same track from two users is kept by each", func(t *testing.T) {
		s := harness.New(t)
		mp3 := audiofile.Generate(t, "song.mp3", audiofile.Spec{Tags: audiofile.SongTags})

		s.Uploaded(alice, s.UploadAudioFile(mp3))
		s.Uploaded(bob, s.UploadAudioFile(mp3))

		assert.Equal(t, []string{"Artist/Album/01 - Dup Song.mp3"}, s.PersonalFiles(alice))
		assert.Equal(t, []string{"Artist/Album/01 - Dup Song.mp3"}, s.PersonalFiles(bob))
		assert.Empty(t, s.Telegram.Replies(t))
	})

	t.Run("each user has their own Inbox", func(t *testing.T) {
		s := harness.New(t)
		untagged := audiofile.Generate(t, "noise.mp3", audiofile.Spec{})

		s.Send(s.AudioMessage(alice, s.UploadAudioFile(untagged)))
		s.Send(s.AudioMessage(bob, s.UploadAudioFile(untagged)))
		s.WaitIngest()

		assert.Equal(t, []string{"Inbox/noise.mp3"}, s.PersonalFiles(alice))
		assert.Equal(t, []string{"Inbox/noise.mp3"}, s.PersonalFiles(bob))
	})
}
