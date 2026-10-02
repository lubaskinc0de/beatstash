package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/telegram"
)

func TestMusic(t *testing.T) {
	t.Parallel()

	t.Run("music opens on the shared tab", func(t *testing.T) {
		s := harness.New(t)
		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.ShareTrack(alice, fixtureButton)

		s.Open(bob, s.Catalog(bob).MusicButton())

		c := s.Catalog(bob)
		buttons := telegram.ButtonTexts(s.Telegram.Buttons(t))
		assert.Contains(t, s.WindowText(), c.FeedTitle())
		assert.Contains(t, buttons, fixtureButton)
		assert.Contains(t, buttons, c.MineTab())
	})

	t.Run("shared track opens its card", func(t *testing.T) {
		s := harness.New(t)
		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.ShareTrack(alice, fixtureButton)

		s.Open(bob, s.Catalog(bob).MusicButton(), fixtureButton)

		c := s.Catalog(bob)
		assert.Contains(t, s.WindowText(), "@alice")
		assert.Equal(t, []string{c.TakeButton(), c.SendFileButton(), c.Back()}, telegram.ButtonTexts(s.Telegram.Buttons(t)))
	})

	t.Run("card leads back to the shared tab", func(t *testing.T) {
		s := harness.New(t)
		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.ShareTrack(alice, fixtureButton)

		s.Open(bob, s.Catalog(bob).MusicButton(), fixtureButton, s.Catalog(bob).Back())

		assert.Contains(t, s.WindowText(), s.Catalog(bob).FeedTitle())
	})

	t.Run("mine tab lists own tracks and leads back to shared", func(t *testing.T) {
		s := harness.New(t)
		s.Uploaded(alice, s.UploadAudio("track.mp3"))

		s.Open(alice, s.Catalog(alice).MusicButton(), s.Catalog(alice).MineTab())
		mine := s.WindowText()
		s.Go(alice, s.Catalog(alice).SharedTab())

		c := s.Catalog(alice)
		assert.Contains(t, mine, c.ShareScreen("", true))
		assert.Contains(t, s.WindowText(), c.FeedEmpty())
	})

	t.Run("top opens from the shared tab and leads back to it", func(t *testing.T) {
		s := harness.New(t)

		s.Open(alice, s.Catalog(alice).MusicButton(), s.Catalog(alice).TopButton())
		top := s.WindowText()
		s.Go(alice, s.Catalog(alice).Back())

		c := s.Catalog(alice)
		assert.Contains(t, top, c.TopNobody())
		assert.Contains(t, s.WindowText(), c.FeedEmpty())
	})
}
