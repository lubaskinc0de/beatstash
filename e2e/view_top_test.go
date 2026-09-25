package e2e

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/audiofile"
)

const (
	mostShared = "Больше всех расшарил"
	mostTaken  = "Чаще всего берут"
	allTime    = "за всё время"
	thisMonth  = "за этот месяц"
)

func TestTop(t *testing.T) {
	t.Parallel()

	t.Run("album counts as its tracks", func(t *testing.T) {
		s := harness.New(t)
		s.Share(alice, s.UploadAlbum(alice, "Album", 3)[0], "💿 Альбом целиком")
		s.Share(bob, s.Uploaded(bob, s.UploadAudio("track.mp3")), "🔗 Трек")

		top := top(s, alice)

		assert.Equal(t, []string{"1. @alice — 3", "2. @bob — 1"}, topLines(t, top, mostShared, allTime))
		assert.Equal(t, []string{"1. @alice — 3", "2. @bob — 1"}, topLines(t, top, mostShared, thisMonth))
	})

	t.Run("author taken by two users ranks first", func(t *testing.T) {
		s := harness.New(t)
		s.Share(bob, s.Uploaded(bob, s.UploadAudioFile(audiofile.Generate(t, "song.mp3", audiofile.Spec{Tags: audiofile.SongTags}))), "🔗 Трек")
		s.Share(alice, s.Uploaded(alice, s.UploadAudio("track.mp3")), "🔗 Трек")
		s.Take(bob, 1)
		s.Take(admin, 1)

		top := top(s, alice)

		assert.Equal(t, []string{"1. @alice — 2"}, topLines(t, top, mostTaken, allTime))
	})

	t.Run("last month's Share counts only for all time", func(t *testing.T) {
		s := harness.New(t)
		s.Share(alice, s.Uploaded(alice, s.UploadAudio("track.mp3")), "🔗 Трек")
		s.Clock.Advance(32 * 24 * time.Hour)
		s.Share(bob, s.Uploaded(bob, s.UploadAudioFile(audiofile.Generate(t, "song.mp3", audiofile.Spec{Tags: audiofile.SongTags}))), "🔗 Трек")

		top := top(s, alice)

		assert.Equal(t, []string{"1. @alice — 1", "2. @bob — 1"}, topLines(t, top, mostShared, allTime))
		assert.Equal(t, []string{"1. @bob — 1"}, topLines(t, top, mostShared, thisMonth))
	})

	t.Run("taking one's own Share does not count", func(t *testing.T) {
		s := harness.New(t)
		s.Share(alice, s.Uploaded(alice, s.UploadAudio("track.mp3")), "🔗 Трек")

		s.Take(alice, 1)

		assert.Equal(t, []string{"Пока никого"}, topLines(t, top(s, alice), mostTaken, allTime))
	})

	t.Run("upload of a shared file does not count as Take", func(t *testing.T) {
		s := harness.New(t)
		audio := s.UploadAudio("track.mp3")
		s.Share(alice, s.Uploaded(alice, audio), "🔗 Трек")

		s.Uploaded(bob, audio)

		assert.Equal(t, []string{"Пока никого"}, topLines(t, top(s, alice), mostTaken, allTime))
	})

	t.Run("inline top is ready to send", func(t *testing.T) {
		s := harness.New(t)
		s.Share(alice, s.Uploaded(alice, s.UploadAudio("track.mp3")), "🔗 Трек")
		query := s.InlineQuery(bob, "top")

		s.Send(query)

		results := s.Telegram.InlineAnswerTo(t, query).Results
		require.Len(t, results, 1)
		assert.Contains(t, results[0].Title, "Top")
	})
}

func top(s *harness.Scenario, user harness.User) string {
	s.Send(s.TextMessage(user, "/top"))
	return s.LastReply().Text
}

// topLines returns the rows of one rating, the block titled with its name and period.
func topLines(t *testing.T, top, rating, period string) []string {
	t.Helper()

	for _, block := range strings.Split(top, "\n\n") {
		lines := strings.Split(strings.TrimSpace(block), "\n")
		if strings.Contains(lines[0], rating) && strings.Contains(lines[0], period) {
			return lines[1:]
		}
	}
	t.Fatalf("no %q %q rating in %q", rating, period, top)
	return nil
}
