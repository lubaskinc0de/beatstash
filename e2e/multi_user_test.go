package e2e

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNavidromeAccount(t *testing.T) {
	t.Run("each user sees their own np", func(t *testing.T) {
		s := newScenario(t)
		aliceAccount := env.navidrome.createAccount(t, "alice")
		bobAccount := env.navidrome.createAccount(t, "bob")
		s.link(alice, aliceAccount)
		s.link(bob, bobAccount)
		aliceAudio := s.uploadAudio("track.mp3")
		bobAudio := s.uploadAudioFile(makeAudio(t, "other.mp3", audioSpec{Tags: map[string]string{
			"artist": "Other Artist", "album": "Other Album", "title": "Other Song",
		}}))
		s.send(s.audioMessage(alice, aliceAudio))
		s.send(s.audioMessage(bob, bobAudio))
		env.navidrome.startPlaying(t, aliceAccount, env.navidrome.indexedTrack(t, aliceAccount, s.library, fixtureTitle).ID)
		env.navidrome.startPlaying(t, bobAccount, env.navidrome.indexedTrack(t, bobAccount, s.library, "Other Song").ID)
		aliceQuery := s.inlineQuery(alice, "np")
		bobQuery := s.inlineQuery(bob, "np")

		s.send(aliceQuery)
		s.send(bobQuery)

		aliceAnswer := s.botAPI.inlineAnswerTo(t, aliceQuery)
		require.Len(t, aliceAnswer.Results, 1)
		assert.Equal(t, aliceAudio.FileID, aliceAnswer.Results[0].AudioFileID)
		bobAnswer := s.botAPI.inlineAnswerTo(t, bobQuery)
		require.Len(t, bobAnswer.Results, 1)
		assert.Equal(t, bobAudio.FileID, bobAnswer.Results[0].AudioFileID)
	})

	t.Run("link deletes the password message and confirms", func(t *testing.T) {
		s := newScenario(t)
		account := env.navidrome.createAccount(t, "alice")

		link := s.link(alice, account)

		assert.Equal(t, []string{strconv.Itoa(link.Message.ID)}, s.botAPI.deletedMessages())
		assert.Contains(t, lastReply(t, s).Text, "привязан")
	})

	t.Run("recent shows the linked account's history", func(t *testing.T) {
		s := newScenario(t)
		account := env.navidrome.createAccount(t, "alice")
		s.link(alice, account)
		audio := s.uploadAudio("track.mp3")
		s.send(s.audioMessage(alice, audio))
		env.navidrome.play(t, account, env.navidrome.indexedTrack(t, account, s.library, fixtureTitle).ID)
		query := s.inlineQuery(alice, "recent")

		s.send(query)

		assert.Contains(t, audioFileIDs(s.botAPI.inlineAnswerTo(t, query)), audio.FileID)
	})

	t.Run("link with wrong password is rejected", func(t *testing.T) {
		s := newScenario(t)
		account := env.navidrome.createAccount(t, "alice")
		account.Password = "wrong"

		link := s.link(alice, account)
		query := s.inlineQuery(alice, "np")
		s.send(query)

		assert.Equal(t, []string{strconv.Itoa(link.Message.ID)}, s.botAPI.deletedMessages())
		assert.Contains(t, lastReply(t, s).Text, "Неверный логин или пароль")
		assertLinkHint(t, s.botAPI.inlineAnswerTo(t, query))
	})

	t.Run("inline without account suggests linking", func(t *testing.T) {
		s := newScenario(t)
		np := s.inlineQuery(alice, "np")
		recent := s.inlineQuery(alice, "recent")

		s.send(np)
		s.send(recent)

		assertLinkHint(t, s.botAPI.inlineAnswerTo(t, np))
		assertLinkHint(t, s.botAPI.inlineAnswerTo(t, recent))
	})
}

func lastReply(t *testing.T, s *scenario) reply {
	t.Helper()

	replies := s.botAPI.replies(t)
	require.NotEmpty(t, replies)
	return replies[len(replies)-1]
}

func audioFileIDs(answer inlineAnswer) []string {
	var ids []string
	for _, result := range answer.Results {
		if result.AudioFileID != "" {
			ids = append(ids, result.AudioFileID)
		}
	}
	return ids
}

func assertLinkHint(t *testing.T, answer inlineAnswer) {
	t.Helper()

	require.Len(t, answer.Results, 1)
	assert.Contains(t, answer.Results[0].Title, "Привяжите аккаунт Navidrome")
}
