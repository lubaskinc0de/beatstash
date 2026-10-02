package e2e

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/audiofile"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/view_top"
)

func TestTop(t *testing.T) {
	t.Parallel()

	t.Run("album counts as its tracks", func(t *testing.T) {
		s := harness.New(t)
		s.UploadAlbum(alice, "Album", 3)
		s.ShareAlbum(alice, uploadedAlbum("Album"))
		s.Uploaded(bob, s.UploadAudio("track.mp3"))
		s.ShareTrack(bob, fixtureButton)

		top := top(s, alice)

		assert.Equal(t, []string{"1. @alice — 3", "2. @bob — 1"}, topLines(t, top, s.Catalog(alice).TopSharedLabel(), s.Catalog(alice).TopAllTimeLabel()))
		assert.Equal(t, []string{"1. @alice — 3", "2. @bob — 1"}, topLines(t, top, s.Catalog(alice).TopSharedLabel(), s.Catalog(alice).TopThisMonthLabel()))
	})

	t.Run("author taken by two users ranks first", func(t *testing.T) {
		s := harness.New(t)
		s.Uploaded(bob, s.UploadAudioFile(audiofile.Generate(t, "song.mp3", audiofile.Spec{Tags: audiofile.SongTags})))
		s.ShareTrack(bob, dupSongButton)
		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.ShareTrack(alice, fixtureButton)
		s.Take(bob, 1)
		s.Take(admin, 1)

		top := top(s, alice)

		assert.Equal(t, []string{"1. @alice — 2"}, topLines(t, top, s.Catalog(alice).TopTakenLabel(), s.Catalog(alice).TopAllTimeLabel()))
	})

	t.Run("last month's Share counts only for all time", func(t *testing.T) {
		s := harness.New(t)
		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.ShareTrack(alice, fixtureButton)
		s.Clock.Advance(32 * 24 * time.Hour)
		s.Uploaded(bob, s.UploadAudioFile(audiofile.Generate(t, "song.mp3", audiofile.Spec{Tags: audiofile.SongTags})))
		s.ShareTrack(bob, dupSongButton)

		top := top(s, alice)

		assert.Equal(t, []string{"1. @alice — 1", "2. @bob — 1"}, topLines(t, top, s.Catalog(alice).TopSharedLabel(), s.Catalog(alice).TopAllTimeLabel()))
		assert.Equal(t, []string{"1. @bob — 1"}, topLines(t, top, s.Catalog(alice).TopSharedLabel(), s.Catalog(alice).TopThisMonthLabel()))
	})

	t.Run("taking one's own Share does not count", func(t *testing.T) {
		s := harness.New(t)
		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.ShareTrack(alice, fixtureButton)

		s.OpenShared(alice, 1)
		s.Go(alice, s.Catalog(alice).InLibraryButton())

		assert.Equal(t, []string{s.Catalog(alice).TopNobody()}, topLines(t, top(s, alice), s.Catalog(alice).TopTakenLabel(), s.Catalog(alice).TopAllTimeLabel()))
	})

	t.Run("upload of a shared file does not count as Take", func(t *testing.T) {
		s := harness.New(t)
		audio := s.UploadAudio("track.mp3")
		s.Uploaded(alice, audio)
		s.ShareTrack(alice, fixtureButton)

		s.Uploaded(bob, audio)

		assert.Equal(t, []string{s.Catalog(alice).TopNobody()}, topLines(t, top(s, alice), s.Catalog(alice).TopTakenLabel(), s.Catalog(alice).TopAllTimeLabel()))
	})

	t.Run("inline top is ready to send", func(t *testing.T) {
		s := harness.New(t)
		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.ShareTrack(alice, fixtureButton)
		query := s.InlineQuery(bob, "top")

		s.Send(query)

		results := s.Telegram.InlineAnswerTo(t, query).Results
		require.Len(t, results, 1)
		assert.Contains(t, results[0].Title, s.Catalog(bob).TopArticle(&view_top.Top{}).Title)
	})
}

func top(s *harness.Scenario, user harness.User) string {
	s.Open(user, s.Catalog(user).MusicButton(), s.Catalog(user).TopButton())
	return s.WindowText()
}

// topLines returns the rows of the rating identified by its localized heading.
func topLines(t *testing.T, top, rating, period string) []string {
	t.Helper()

	blocks := strings.Split(top, "\n\n")
	require.Len(t, blocks, 5)
	for _, block := range blocks[1:] {
		lines := strings.Split(strings.TrimSpace(block), "\n")
		if strings.Contains(lines[0], rating) && strings.Contains(lines[0], period) {
			return lines[1:]
		}
	}
	t.Fatalf("no %q %q rating in %q", rating, period, top)
	return nil
}
