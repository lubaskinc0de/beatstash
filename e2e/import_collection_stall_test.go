package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/zvuk"
)

func TestZvukStall(t *testing.T) {
	t.Parallel()

	t.Run("silent download is cut off and the next attempt brings the track", func(t *testing.T) {
		s := harness.New(t, harness.WithStallTimeout(200*time.Millisecond))
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukSong("101", "Only Song", false)
		s.LikeOnZvuk("101")
		s.Zvuk.StallStream("101")

		s.ImportZvuk(alice)
		s.WaitIngest()

		assert.Equal(t, []string{"Zvuk Band/Zvuk Album (2021)/01 - Only Song.mp3"}, s.PersonalFiles(alice))
		assert.Len(t, s.Zvuk.EventsOf(zvuk.StreamAsked), 2)
		assert.Len(t, s.SentMessagesContaining(alice, importSummary(s, alice, 1, 1)), 1)
	})

	t.Run("slow but steady download is not cut off", func(t *testing.T) {
		s := harness.New(t, harness.WithStallTimeout(300*time.Millisecond))
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukSong("101", "Only Song", false)
		s.LikeOnZvuk("101")
		s.Zvuk.TrickleStream("101", 100*time.Millisecond)

		s.ImportZvuk(alice)
		s.WaitIngest()

		assert.Equal(t, []string{"Zvuk Band/Zvuk Album (2021)/01 - Only Song.mp3"}, s.PersonalFiles(alice))
		assert.Len(t, s.Zvuk.EventsOf(zvuk.StreamAsked), 1)
	})
}
