package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFirstContact(t *testing.T) {
	t.Run("new user starting with inline np gets an answer", func(t *testing.T) {
		s := newScenario(t)
		query := s.inlineQuery(alice, "np")

		s.send(query)

		answers := s.botAPI.InlineAnswers(t)
		require.Len(t, answers, 1)
		assert.Equal(t, query.InlineQuery.ID, answers[0].QueryID)
	})

	t.Run("new user pressing a button gets an answer", func(t *testing.T) {
		s := newScenario(t)
		press := s.callbackQuery(alice, "any")

		s.send(press)

		assert.Equal(t, []string{press.CallbackQuery.ID}, s.botAPI.AnsweredCallbacks())
	})
}
