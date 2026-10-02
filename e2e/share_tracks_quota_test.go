package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/audiofile"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

func TestTakeQuota(t *testing.T) {
	t.Parallel()

	t.Run("Take over the taker's Quota is rejected", func(t *testing.T) {
		own := audiofile.Generate(t, "own.mp3", audiofile.Spec{Tags: map[string]string{"artist": "B", "title": "Own"}})
		ownSize, takenSize := harness.FileSize(t, own), harness.FileSize(t, audiofile.Fixture("track.mp3"))
		quota := library.Quota(max(ownSize, takenSize) + min(ownSize, takenSize)/2)
		s := harness.New(t, harness.WithDefaultQuota(quota), harness.WithAdminContact(adminContact))
		s.Uploaded(bob, s.UploadAudioFile(own))
		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.ShareTrack(alice, fixtureButton)

		s.Take(bob, 1)

		used := harness.FileSize(t, s.PersonalPath(bob, "B/Singles/Own.mp3"))
		assert.Contains(t, s.Telegram.CallbackAnswers(), s.Catalog(bob).NoRoom(library.Usage{Used: used, Quota: quota}, adminContact))
		assert.Equal(t, []string{"B/Singles/Own.mp3"}, s.PersonalFiles(bob))
	})
}

func TestShareQuota(t *testing.T) {
	t.Parallel()

	t.Run("Share over the Shared Library's Quota is rejected", func(t *testing.T) {
		s := harness.New(t, harness.WithSharedQuota(20*kb), harness.WithAdminContact(adminContact))
		s.Uploaded(alice, s.UploadAudio("track.mp3"))

		s.ShareTrack(alice, fixtureButton)

		assert.Equal(t, s.Catalog(alice).SharedLibraryFull(adminContact), s.LastCallbackAnswer())
		assert.Empty(t, s.SharedFiles())
	})

	t.Run("album that fits only partly shares none of its tracks", func(t *testing.T) {
		song := audiofile.Generate(t, "song1.mp3", audiofile.Spec{Tags: map[string]string{
			"artist": "Artist", "album": "Album", "track": "1", "title": "Song 1",
		}})
		s := harness.New(t, harness.WithSharedQuota(library.Quota(harness.FileSize(t, song)*5/2)))
		s.UploadAlbum(alice, "Album", 3)

		s.ShareAlbum(alice, uploadedAlbum("Album"))

		assert.Equal(t, s.Catalog(alice).SharedLibraryFull(""), s.LastCallbackAnswer())
		assert.Empty(t, s.SharedFiles())
	})
}
