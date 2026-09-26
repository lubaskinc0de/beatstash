package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
)

func TestFirstContact(t *testing.T) {
	t.Parallel()

	t.Run("new user starting with inline np gets an answer", func(t *testing.T) {
		s := harness.New(t)
		query := s.InlineQuery(alice, "np")

		s.Send(query)

		answers := s.Telegram.InlineAnswers(t)
		require.Len(t, answers, 1)
		assert.Equal(t, query.InlineQuery.ID, answers[0].QueryID)
	})

	t.Run("new user pressing a button gets an answer", func(t *testing.T) {
		s := harness.New(t)
		press := s.CallbackQuery(alice, "any")

		s.Send(press)

		assert.Equal(t, []string{press.CallbackQuery.ID}, s.Telegram.AnsweredCallbacks())
	})
}
