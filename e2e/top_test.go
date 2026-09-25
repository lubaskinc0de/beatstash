package e2e

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		s := newScenario(t)
		s.share(alice, s.uploadAlbum(alice, "Album", 3)[0], "💿 Альбом целиком")
		s.share(bob, s.uploaded(bob, s.uploadAudio("track.mp3")), "🔗 Трек")

		top := s.top(alice)

		assert.Equal(t, []string{"1. @alice — 3", "2. @bob — 1"}, topLines(t, top, mostShared, allTime))
		assert.Equal(t, []string{"1. @alice — 3", "2. @bob — 1"}, topLines(t, top, mostShared, thisMonth))
	})

	t.Run("author taken by two users ranks first", func(t *testing.T) {
		s := newScenario(t)
		s.share(bob, s.uploaded(bob, s.uploadAudioFile(makeAudio(t, "song.mp3", audioSpec{Tags: songTags}))), "🔗 Трек")
		s.share(alice, s.uploaded(alice, s.uploadAudio("track.mp3")), "🔗 Трек")
		s.take(bob, 1)
		s.take(adminUser, 1)

		top := s.top(alice)

		assert.Equal(t, []string{"1. @alice — 2"}, topLines(t, top, mostTaken, allTime))
	})

	t.Run("last month's Share counts only for all time", func(t *testing.T) {
		s := newScenario(t)
		s.share(alice, s.uploaded(alice, s.uploadAudio("track.mp3")), "🔗 Трек")
		s.clock.advance(32 * 24 * time.Hour)
		s.share(bob, s.uploaded(bob, s.uploadAudioFile(makeAudio(t, "song.mp3", audioSpec{Tags: songTags}))), "🔗 Трек")

		top := s.top(alice)

		assert.Equal(t, []string{"1. @alice — 1", "2. @bob — 1"}, topLines(t, top, mostShared, allTime))
		assert.Equal(t, []string{"1. @bob — 1"}, topLines(t, top, mostShared, thisMonth))
	})

	t.Run("taking one's own Share does not count", func(t *testing.T) {
		s := newScenario(t)
		s.share(alice, s.uploaded(alice, s.uploadAudio("track.mp3")), "🔗 Трек")

		s.take(alice, 1)

		assert.Equal(t, []string{"Пока никого"}, topLines(t, s.top(alice), mostTaken, allTime))
	})

	t.Run("upload of a shared file does not count as Take", func(t *testing.T) {
		s := newScenario(t)
		audio := s.uploadAudio("track.mp3")
		s.share(alice, s.uploaded(alice, audio), "🔗 Трек")

		s.uploaded(bob, audio)

		assert.Equal(t, []string{"Пока никого"}, topLines(t, s.top(alice), mostTaken, allTime))
	})

	t.Run("inline top is ready to send", func(t *testing.T) {
		s := newScenario(t)
		s.share(alice, s.uploaded(alice, s.uploadAudio("track.mp3")), "🔗 Трек")
		query := s.inlineQuery(bob, "top")

		s.send(query)

		results := s.botAPI.inlineAnswerTo(t, query).Results
		require.Len(t, results, 1)
		assert.Contains(t, results[0].Title, "Top")
	})
}

func (s *scenario) top(user telegramUser) string {
	s.t.Helper()

	s.send(s.textMessage(user, "/top"))
	return lastReply(s.t, s).Text
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
