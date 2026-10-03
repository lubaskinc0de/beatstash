package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lubaskinc0de/beatstash/e2e/harness"
	"github.com/lubaskinc0de/beatstash/e2e/harness/audiofile"
	"github.com/lubaskinc0de/beatstash/internal/application/share_tracks"
	"github.com/lubaskinc0de/beatstash/internal/domain/access"
)

func TestShare(t *testing.T) {
	t.Parallel()

	t.Run("shared track is found by another user", func(t *testing.T) {
		s := harness.New(t)
		bobAccount := s.LinkNewAccount(bob)
		s.Uploaded(alice, s.UploadAudio("track.mp3"))

		s.ShareTrack(alice, fixtureButton)

		assert.Equal(t, []string{audiofile.FixtureTrackPath}, s.SharedFiles())
		s.Navidrome.IndexedTrack(t, bobAccount, s.Library, audiofile.FixtureTitle)
		assert.Contains(t, s.LastCallbackAnswer(), s.Catalog(alice).ShareResult(&share_tracks.ShareResult{Created: 1}))
	})

	t.Run("unsharing an album removes its tracks from the Shared Library", func(t *testing.T) {
		s := harness.New(t)
		s.UploadAlbum(alice, "Album", 3)
		s.ShareAlbum(alice, uploadedAlbum("Album"))

		s.UnshareAlbum(alice, uploadedAlbum("Album"))

		assert.Empty(t, s.SharedFiles())
		assert.Equal(t, []string{
			"Artist/Album/01 - Song 1.mp3",
			"Artist/Album/02 - Song 2.mp3",
			"Artist/Album/03 - Song 3.mp3",
		}, s.PersonalFiles(alice))
	})

	t.Run("unsharing an album keeps a track shared by another user", func(t *testing.T) {
		s := harness.New(t)
		s.UploadAlbum(alice, "Album", 3)
		s.ShareAlbum(alice, uploadedAlbum("Album"))
		s.UploadAlbum(bob, "Album", 3)
		s.ShareTrack(bob, "Artist — Song 1")

		s.UnshareAlbum(alice, uploadedAlbum("Album"))

		assert.Equal(t, []string{"Artist/Album/01 - Song 1.mp3"}, s.SharedFiles())
		assert.Equal(t, []string{
			"Artist/Album/01 - Song 1.mp3",
			"Artist/Album/02 - Song 2.mp3",
			"Artist/Album/03 - Song 3.mp3",
		}, s.PersonalFiles(alice))
		assert.Equal(t, []string{
			"Artist/Album/01 - Song 1.mp3",
			"Artist/Album/02 - Song 2.mp3",
			"Artist/Album/03 - Song 3.mp3",
		}, s.PersonalFiles(bob))
	})

	t.Run("sharing an album reports tracks already shared by another user", func(t *testing.T) {
		s := harness.New(t)
		s.UploadAlbum(bob, "Album", 3)
		s.ShareTrack(bob, "Artist — Song 2")
		s.UploadAlbum(alice, "Album", 3)

		s.ShareAlbum(alice, uploadedAlbum("Album"))

		assert.Contains(t, s.LastCallbackAnswer(), s.Catalog(alice).ShareResult(&share_tracks.ShareResult{Created: 2, AlreadyShared: 1}))
		assert.Equal(t, []string{
			"Artist/Album/01 - Song 1.mp3",
			"Artist/Album/02 - Song 2.mp3",
			"Artist/Album/03 - Song 3.mp3",
		}, s.SharedFiles())
	})

	t.Run("unshared track disappears for others", func(t *testing.T) {
		s := harness.New(t)
		bobAccount := s.LinkNewAccount(bob)
		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.ShareTrack(alice, fixtureButton)
		s.Navidrome.IndexedTrack(t, bobAccount, s.Library, audiofile.FixtureTitle)

		s.UnshareTrack(alice, fixtureButton)

		assert.Empty(t, s.SharedFiles())
		assert.Equal(t, []string{audiofile.FixtureTrackPath}, s.PersonalFiles(alice))
		s.Navidrome.UntilGone(t, bobAccount, s.Library, audiofile.FixtureTitle)
	})

	t.Run("second sharer learns who shared first", func(t *testing.T) {
		s := harness.New(t)
		mp3 := audiofile.Generate(t, "song.mp3", audiofile.Spec{Tags: audiofile.SongTags})
		s.Uploaded(alice, s.UploadAudioFile(mp3))
		s.ShareTrack(alice, dupSongButton)
		s.Uploaded(bob, s.UploadAudioFile(mp3))

		s.ShareTrack(bob, dupSongButton)

		assert.Contains(t, s.LastCallbackAnswer(), s.Catalog(bob).ShareResult(&share_tracks.ShareResult{
			AlreadyShared: 1, Author: &access.User{Username: alice.Username},
		}))
		assert.Equal(t, []string{"Artist/Album/01 - Dup Song.mp3"}, s.SharedFiles())
	})

	t.Run("track stays shared while its second sharer keeps it", func(t *testing.T) {
		s := harness.New(t)
		mp3 := audiofile.Generate(t, "song.mp3", audiofile.Spec{Tags: audiofile.SongTags})
		s.Uploaded(alice, s.UploadAudioFile(mp3))
		s.ShareTrack(alice, dupSongButton)
		s.Uploaded(bob, s.UploadAudioFile(mp3))
		s.ShareTrack(bob, dupSongButton)

		s.UnshareTrack(alice, dupSongButton)

		assert.Equal(t, []string{"Artist/Album/01 - Dup Song.mp3"}, s.SharedFiles())
	})

	t.Run("Inbox track goes by its file name and cannot be shared", func(t *testing.T) {
		s := harness.New(t)
		s.Uploaded(alice, s.UploadAudioFile(audiofile.Generate(t, "audio_1.mp3", audiofile.Spec{})))

		s.ShareTrack(alice, "audio_1.mp3")

		assert.Contains(t, s.WindowText(), "🎧 audio_1.mp3")
		assert.Equal(t, s.Catalog(alice).InboxNotShareable(), s.LastCallbackAnswer())
		assert.Empty(t, s.SharedFiles())
	})

	t.Run("share command is gone", func(t *testing.T) {
		s := harness.New(t)
		upload := s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.Telegram.Forget()
		share := s.ReplyCommand(alice, "/share", upload)

		s.Send(share)

		assertOnlyDeleted(t, s, share)
		assert.Empty(t, s.SharedFiles())
	})
}
