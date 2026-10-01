package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
)

func TestScratch(t *testing.T) {
	t.Parallel()

	t.Run("second instance leaves a download in progress alone", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukSong("101", "Only Song", false)
		s.LikeOnZvuk("101")
		stall := s.Zvuk.StallStream("101")
		s.ImportZvuk(alice)
		<-stall.Arrived()

		s.StartReplica()
		stall.Resume()
		s.WaitIngest()

		assert.Equal(t, []string{"Zvuk Band/Zvuk Album (2021)/01 - Only Song.mp3"}, s.PersonalFiles(alice))
		assert.Len(t, s.SentMessagesContaining(alice, importSummary(s, alice, 1, 1)), 1)
	})

	t.Run("start removes scratch files older than scratch_ttl", func(t *testing.T) {
		s := harness.New(t)
		s.PutScratchFile("stale.mp3", 2*time.Hour)
		s.PutScratchFile("fresh.mp3", time.Minute)

		s.Restart()

		assert.Equal(t, []string{"fresh.mp3"}, s.ScratchFiles())
	})

	t.Run("Navidrome library in the scratch folder is not given to new accounts", func(t *testing.T) {
		var scratch harness.NavidromeLibrary
		s := harness.NewOwnNavidrome(t, func(s *harness.Scenario) {
			scratch = s.NewScratchLibrary("inner")
			s.Navidrome.GiveToNewAccounts(t, scratch.ID)
		})

		account := s.Navidrome.CreateAccountWithDefaults(t, "dave")

		assert.NotContains(t, s.Navidrome.Libraries(t, account), scratch.Path)
	})
}
