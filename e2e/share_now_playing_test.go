package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (s *scenario) nowPlayingButtons(user telegramUser) []button {
	s.t.Helper()

	query := s.inlineQuery(user, "np")
	s.send(query)
	results := s.botAPI.inlineAnswerTo(s.t, query).Results
	require.Len(s.t, results, 1)
	return results[0].buttons()
}

func TestShareNowPlaying(t *testing.T) {
	t.Run("np shares the user's own playing track", func(t *testing.T) {
		s := newScenario(t)
		account := s.linkNewAccount(alice)
		s.uploaded(alice, s.uploadAudio("track.mp3"))
		env.navidrome.startPlaying(t, account, env.navidrome.indexedTrack(t, account, s.library, fixtureTitle).ID)
		share := buttonIn(t, s.nowPlayingButtons(alice), "🔗 Share")

		s.pressInline(alice, share)

		assert.Equal(t, []string{fixtureTrackPath}, s.sharedFiles())
		assert.Contains(t, lastCallbackAnswer(t, s), "В общей библиотеке")
	})

	t.Run("np of a shared track has no Share button", func(t *testing.T) {
		s := newScenario(t)
		account := s.linkNewAccount(alice)
		s.share(alice, s.uploaded(alice, s.uploadAudio("track.mp3")), "🔗 Трек")
		env.navidrome.startPlaying(t, account, env.navidrome.indexedTrack(t, account, s.personalPath(alice, ""), fixtureTitle).ID)
		query := s.inlineQuery(alice, "np")

		s.send(query)

		results := s.botAPI.inlineAnswerTo(t, query).Results
		require.Len(t, results, 1)
		assert.Empty(t, results[0].buttons())
	})

	t.Run("np of another user's shared track has no Share button", func(t *testing.T) {
		s := newScenario(t)
		bobAccount := s.linkNewAccount(bob)
		s.share(alice, s.uploaded(alice, s.uploadAudio("track.mp3")), "🔗 Трек")
		env.navidrome.startPlaying(t, bobAccount, env.navidrome.indexedTrack(t, bobAccount, s.library, fixtureTitle).ID)
		query := s.inlineQuery(bob, "np")

		s.send(query)

		results := s.botAPI.inlineAnswerTo(t, query).Results
		require.Len(t, results, 1)
		assert.Empty(t, results[0].buttons())
	})
}
