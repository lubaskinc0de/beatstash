package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/lubaskinc0de/beatstash/e2e/harness"
	"github.com/lubaskinc0de/beatstash/e2e/harness/zvuk"
)

const stallTimeout = time.Minute

func TestZvukStall(t *testing.T) {
	t.Parallel()

	t.Run("silent download is cut off and the next attempt brings the track", func(t *testing.T) {
		s := harness.New(t, harness.WithStallTimeout(stallTimeout))
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukSong("101", "Only Song", false)
		s.LikeOnZvuk("101")
		stall := s.Zvuk.StallStream("101")

		s.ImportZvuk(alice)
		<-stall.Arrived()
		<-s.Clock.Armed()
		s.Clock.Advance(stallTimeout)
		s.WaitIngest()

		assert.Equal(t, []string{"Zvuk Band/Zvuk Album (2021)/01 - Only Song.mp3"}, s.PersonalFiles(alice))
		assert.Len(t, s.Zvuk.EventsOf(zvuk.StreamAsked), 2)
		assert.Len(t, s.SentMessagesContaining(alice, importSummary(s, alice, 1, 1)), 1)
	})

	t.Run("slow but steady download is not cut off", func(t *testing.T) {
		s := harness.New(t, harness.WithStallTimeout(stallTimeout))
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukSong("101", "Only Song", false)
		s.LikeOnZvuk("101")
		trickle := s.Zvuk.TrickleStream("101")

		s.ImportZvuk(alice)
		for range zvuk.TricklePieces {
			trickle.Send()
			<-s.Clock.Armed()
			s.Clock.Advance(stallTimeout / 2)
		}
		s.WaitIngest()

		assert.Equal(t, []string{"Zvuk Band/Zvuk Album (2021)/01 - Only Song.mp3"}, s.PersonalFiles(alice))
		assert.Len(t, s.Zvuk.EventsOf(zvuk.StreamAsked), 1)
	})
}
