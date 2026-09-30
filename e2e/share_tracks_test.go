package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/audiofile"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/telegram"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/share_tracks"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
)

func TestShareNowPlaying(t *testing.T) {
	t.Parallel()

	t.Run("np shares the user's own playing track", func(t *testing.T) {
		s := harness.New(t)
		account := s.LinkNewAccount(alice)
		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.Navidrome.StartPlaying(t, account, s.Navidrome.IndexedTrack(t, account, s.Library, audiofile.FixtureTitle).ID)
		share := telegram.ButtonNamed(t, s.NowPlaying(alice).Buttons(), s.Catalog(alice).ShareButton())

		s.PressInline(alice, share)

		assert.Equal(t, []string{audiofile.FixtureTrackPath}, s.SharedFiles())
		assert.Contains(t, s.LastCallbackAnswer(), s.Catalog(alice).ShareResult(&share_tracks.ShareResult{Created: 1}))
	})

	t.Run("another user cannot Share a track from Alice's inline np", func(t *testing.T) {
		s := harness.New(t)
		account := s.LinkNewAccount(alice)
		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		track := s.Navidrome.IndexedTrack(t, account, s.PersonalPath(alice, ""), audiofile.FixtureTitle)
		s.Navidrome.StartPlaying(t, account, track.ID)
		share := telegram.ButtonNamed(t, s.NowPlaying(alice).Buttons(), s.Catalog(alice).ShareButton())

		s.PressInline(bob, share)

		assert.Contains(t, s.LastCallbackAnswer(), s.Catalog(bob).NotOwnTrack())
		assert.Empty(t, s.SharedFiles())
	})

	t.Run("np of a shared track has no Share button", func(t *testing.T) {
		s := harness.New(t)
		account := s.LinkNewAccount(alice)
		s.Share(alice, s.Uploaded(alice, s.UploadAudio("track.mp3")), s.Catalog(alice).ShareTrack())
		s.Navidrome.StartPlaying(t, account, s.Navidrome.IndexedTrack(t, account, s.PersonalPath(alice, ""), audiofile.FixtureTitle).ID)
		query := s.InlineQuery(alice, "np")

		s.Send(query)

		results := s.Telegram.InlineAnswerTo(t, query).Results
		require.Len(t, results, 1)
		assert.Empty(t, results[0].Buttons())
	})

	t.Run("np of another user's shared track has no Share button", func(t *testing.T) {
		s := harness.New(t)
		bobAccount := s.LinkNewAccount(bob)
		s.Share(alice, s.Uploaded(alice, s.UploadAudio("track.mp3")), s.Catalog(alice).ShareTrack())
		s.Navidrome.StartPlaying(t, bobAccount, s.Navidrome.IndexedTrack(t, bobAccount, s.Library, audiofile.FixtureTitle).ID)
		query := s.InlineQuery(bob, "np")

		s.Send(query)

		results := s.Telegram.InlineAnswerTo(t, query).Results
		require.Len(t, results, 1)
		assert.Empty(t, results[0].Buttons())
	})
}

func TestShare(t *testing.T) {
	t.Parallel()

	t.Run("shared track is found by another user", func(t *testing.T) {
		s := harness.New(t)
		bobAccount := s.LinkNewAccount(bob)
		upload := s.Uploaded(alice, s.UploadAudio("track.mp3"))

		s.Share(alice, upload, s.Catalog(alice).ShareTrack())

		assert.Equal(t, []string{audiofile.FixtureTrackPath}, s.SharedFiles())
		s.Navidrome.IndexedTrack(t, bobAccount, s.Library, audiofile.FixtureTitle)
		c := s.Catalog(alice)
		assert.Contains(t, s.LastCallbackAnswer(), c.ShareResult(&share_tracks.ShareResult{Created: 1}))
		assert.Equal(t, []string{c.UnshareTrack(), c.UnshareAlbum()}, telegram.ButtonTexts(s.Telegram.Buttons(t)))
	})

	t.Run("shared album puts all its tracks into the Shared Library", func(t *testing.T) {
		s := harness.New(t)
		uploads := s.UploadAlbum(alice, "Album", 3)

		s.Share(alice, uploads[0], s.Catalog(alice).ShareAlbum())

		assert.Equal(t, []string{
			"Artist/Album/01 - Song 1.mp3",
			"Artist/Album/02 - Song 2.mp3",
			"Artist/Album/03 - Song 3.mp3",
		}, s.SharedFiles())
		c := s.Catalog(alice)
		assert.Equal(t, []string{c.UnshareTrack(), c.UnshareAlbum()}, telegram.ButtonTexts(s.Telegram.Buttons(t)))
	})

	t.Run("unsharing an album removes its tracks from the Shared Library", func(t *testing.T) {
		s := harness.New(t)
		uploads := s.UploadAlbum(alice, "Album", 3)
		s.Share(alice, uploads[1], s.Catalog(alice).ShareAlbum())

		s.Press(alice, s.Button(s.Catalog(alice).UnshareAlbum()))

		assert.Empty(t, s.SharedFiles())
		assert.Equal(t, []string{
			"Artist/Album/01 - Song 1.mp3",
			"Artist/Album/02 - Song 2.mp3",
			"Artist/Album/03 - Song 3.mp3",
		}, s.PersonalFiles(alice))
		c := s.Catalog(alice)
		assert.Equal(t, []string{c.ShareTrack(), c.ShareAlbum()}, telegram.ButtonTexts(s.Telegram.Buttons(t)))
	})

	t.Run("unsharing an album keeps a track shared by another user", func(t *testing.T) {
		s := harness.New(t)
		aliceAlbum := s.UploadAlbum(alice, "Album", 3)
		s.Share(alice, aliceAlbum[0], s.Catalog(alice).ShareAlbum())
		unshareAlbum := s.Button(s.Catalog(alice).UnshareAlbum())
		bobAlbum := s.UploadAlbum(bob, "Album", 3)
		s.Share(bob, bobAlbum[0], s.Catalog(bob).ShareTrack())

		s.Press(alice, unshareAlbum)

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
		bobAlbum := s.UploadAlbum(bob, "Album", 3)
		s.Share(bob, bobAlbum[1], s.Catalog(bob).ShareTrack())
		aliceAlbum := s.UploadAlbum(alice, "Album", 3)

		s.Share(alice, aliceAlbum[0], s.Catalog(alice).ShareAlbum())

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
		upload := s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.Share(alice, upload, s.Catalog(alice).ShareTrack())
		s.Navidrome.IndexedTrack(t, bobAccount, s.Library, audiofile.FixtureTitle)

		s.Press(alice, s.Button(s.Catalog(alice).UnshareTrack()))

		assert.Empty(t, s.SharedFiles())
		assert.Equal(t, []string{audiofile.FixtureTrackPath}, s.PersonalFiles(alice))
		s.Navidrome.UntilGone(t, bobAccount, s.Library, audiofile.FixtureTitle)
		c := s.Catalog(alice)
		assert.Equal(t, []string{c.ShareTrack(), c.ShareAlbum()}, telegram.ButtonTexts(s.Telegram.Buttons(t)))
	})

	t.Run("second sharer learns who shared first", func(t *testing.T) {
		s := harness.New(t)
		mp3 := audiofile.Generate(t, "song.mp3", audiofile.Spec{Tags: audiofile.SongTags})
		s.Share(alice, s.Uploaded(alice, s.UploadAudioFile(mp3)), s.Catalog(alice).ShareTrack())
		bobUpload := s.Uploaded(bob, s.UploadAudioFile(mp3))

		s.Share(bob, bobUpload, s.Catalog(bob).ShareTrack())

		assert.Contains(t, s.LastCallbackAnswer(), s.Catalog(bob).ShareResult(&share_tracks.ShareResult{
			AlreadyShared: 1, Author: &access.User{Username: alice.Username},
		}))
		assert.Equal(t, []string{"Artist/Album/01 - Dup Song.mp3"}, s.SharedFiles())
	})

	t.Run("track stays shared while its second sharer keeps it", func(t *testing.T) {
		s := harness.New(t)
		mp3 := audiofile.Generate(t, "song.mp3", audiofile.Spec{Tags: audiofile.SongTags})
		s.Share(alice, s.Uploaded(alice, s.UploadAudioFile(mp3)), s.Catalog(alice).ShareTrack())
		aliceButtons := s.Telegram.Buttons(t)
		s.Share(bob, s.Uploaded(bob, s.UploadAudioFile(mp3)), s.Catalog(bob).ShareTrack())

		s.Press(alice, telegram.ButtonNamed(t, aliceButtons, s.Catalog(alice).UnshareTrack()))

		assert.Equal(t, []string{"Artist/Album/01 - Dup Song.mp3"}, s.SharedFiles())
	})

	t.Run("Inbox track cannot be shared", func(t *testing.T) {
		s := harness.New(t)
		upload := s.Uploaded(alice, s.UploadAudioFile(audiofile.Generate(t, "audio_1.mp3", audiofile.Spec{})))

		s.Send(s.ReplyCommand(alice, "/share", upload))

		assert.Contains(t, s.LastReply().Text, s.Catalog(alice).InboxNotShareable())
		assert.Empty(t, s.Telegram.Buttons(t))
		assert.Empty(t, s.SharedFiles())
	})

	t.Run("share without own audio explains how to use it", func(t *testing.T) {
		s := harness.New(t)
		bobUpload := s.Uploaded(bob, s.UploadAudio("track.mp3"))

		s.Send(s.TextMessage(alice, "/share"))
		s.Send(s.ReplyCommand(alice, "/share", bobUpload))

		replies := s.Telegram.Replies(t)
		require.Len(t, replies, 2)
		assert.Contains(t, replies[0].Text, s.Catalog(alice).ShareUsage())
		assert.Contains(t, replies[1].Text, s.Catalog(alice).ShareUsage())
		assert.Empty(t, s.Telegram.Buttons(t))
	})
}
