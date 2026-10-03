package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lubaskinc0de/beatstash/e2e/harness"
)

func TestButtonColors(t *testing.T) {
	t.Parallel()

	t.Run("main action is blue", func(t *testing.T) {
		s := harness.New(t)

		s.OpenZvuk(alice)

		assert.Equal(t, "primary", s.Button(s.Catalog(alice).Connect()).Style)
	})

	t.Run("take is green", func(t *testing.T) {
		s := harness.New(t)
		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.ShareTrack(alice, fixtureButton)

		s.OpenShared(bob, 1)

		assert.Equal(t, "success", s.Button(s.Catalog(bob).TakeButton()).Style)
	})

	t.Run("disconnect and cancel are red", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)

		s.OpenZvuk(alice)
		disconnect := s.Button(s.Catalog(alice).Disconnect())
		s.Go(alice, s.Catalog(alice).Disconnect(), s.Catalog(alice).Connect())

		assert.Equal(t, "danger", disconnect.Style)
		assert.Equal(t, "danger", s.Button(s.Catalog(alice).Cancel()).Style)
	})

	t.Run("open tab is checked and blue", func(t *testing.T) {
		s := harness.New(t)

		s.Open(alice, s.Catalog(alice).MusicButton())

		c := s.Catalog(alice)
		assert.Equal(t, "primary", s.Button(c.OpenTab(c.MineTab())).Style)
		assert.Empty(t, s.Button(c.SharedTab()).Style)
	})

	t.Run("navigation has no color", func(t *testing.T) {
		s := harness.New(t)

		s.Open(alice, s.Catalog(alice).HelpButton())

		assert.Empty(t, s.Button(s.Catalog(alice).Back()).Style)
		assert.Empty(t, s.Button(s.Catalog(alice).ListenButton()).Style)
	})
}
