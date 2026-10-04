package e2e

import (
	"strconv"
	"testing"
	"time"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/assert"

	"github.com/lubaskinc0de/beatstash/e2e/harness"
	"github.com/lubaskinc0de/beatstash/e2e/harness/telegram"
	"github.com/lubaskinc0de/beatstash/e2e/harness/zvuk"
)

func TestZvukAccount(t *testing.T) {
	t.Parallel()

	t.Run("valid token connects Zvuk", func(t *testing.T) {
		s := harness.New(t)
		s.Zvuk.AddAccount(harness.ZvukToken, true)
		c := s.Catalog(alice)

		msg := s.SendZvukToken(alice, harness.ZvukToken)

		assert.Contains(t, s.Telegram.DeletedMessages(), strconv.Itoa(msg.Message.ID))
		assert.Contains(t, s.WindowText(), c.Connected("zvuk"))
		assert.Contains(t, s.WindowText(), c.Provider("zvuk", "connected"))
	})

	t.Run("invalid token is rejected and leaves Zvuk unconnected", func(t *testing.T) {
		s := harness.New(t)
		c := s.Catalog(alice)

		msg := s.SendZvukToken(alice, "wrong-token")
		rejected := s.WindowText()
		s.OpenZvuk(alice)

		assert.Contains(t, s.Telegram.DeletedMessages(), strconv.Itoa(msg.Message.ID))
		assert.Contains(t, rejected, c.TokenInvalid("zvuk"))
		assert.Contains(t, s.WindowText(), c.Provider("zvuk", "not_connected"))
	})

	t.Run("pending SberPrime subscription with premium connects Zvuk", func(t *testing.T) {
		s := harness.New(t)
		s.Zvuk.AddAccount(harness.ZvukToken, true)
		s.Zvuk.Update(harness.ZvukToken, func(a *zvuk.Account) {
			a.SubscriptionStatus = "pending"
			a.SubscriptionServices = []string{"premium"}
		})
		c := s.Catalog(alice)

		s.SendZvukToken(alice, harness.ZvukToken)

		assert.Contains(t, s.WindowText(), c.Connected("zvuk"))
		assert.Contains(t, s.WindowText(), c.Provider("zvuk", "connected"))
	})

	t.Run("expired pending premium subscription is rejected", func(t *testing.T) {
		s := harness.New(t)
		s.Zvuk.AddAccount(harness.ZvukToken, true)
		s.Zvuk.Update(harness.ZvukToken, func(a *zvuk.Account) {
			a.SubscriptionStatus = "pending"
			a.SubscriptionServices = []string{"premium"}
			a.SubscriptionExpiration = time.Now().Add(-24 * time.Hour).UnixMilli()
		})
		c := s.Catalog(alice)

		s.SendZvukToken(alice, harness.ZvukToken)
		rejected := s.WindowText()
		s.OpenZvuk(alice)

		assert.Contains(t, rejected, c.NoSubscription("zvuk"))
		assert.Contains(t, s.WindowText(), c.Provider("zvuk", "not_connected"))
	})

	t.Run("pending subscription without premium is rejected", func(t *testing.T) {
		s := harness.New(t)
		s.Zvuk.AddAccount(harness.ZvukToken, true)
		s.Zvuk.Update(harness.ZvukToken, func(a *zvuk.Account) {
			a.SubscriptionStatus = "pending"
			a.SubscriptionServices = []string{"other"}
		})
		c := s.Catalog(alice)

		s.SendZvukToken(alice, harness.ZvukToken)
		rejected := s.WindowText()
		s.OpenZvuk(alice)

		assert.Contains(t, rejected, c.NoSubscription("zvuk"))
		assert.Contains(t, s.WindowText(), c.Provider("zvuk", "not_connected"))
	})

	t.Run("account without subscription is rejected and stays unconnected", func(t *testing.T) {
		s := harness.New(t)
		c := s.Catalog(alice)
		s.Zvuk.AddAccount(harness.ZvukToken, false)

		s.SendZvukToken(alice, harness.ZvukToken)
		rejected := s.WindowText()
		s.OpenZvuk(alice)

		assert.Contains(t, rejected, c.NoSubscription("zvuk"))
		assert.Contains(t, s.WindowText(), c.Provider("zvuk", "not_connected"))
	})

	t.Run("user disconnects Zvuk", func(t *testing.T) {
		s := harness.New(t)
		c := s.Catalog(alice)
		s.ConnectZvuk(alice, harness.ZvukToken)

		s.OpenZvuk(alice, s.Catalog(alice).Disconnect())

		assert.Contains(t, s.WindowText(), c.Disconnected("zvuk"))
		assert.Contains(t, s.WindowText(), c.Provider("zvuk", "not_connected"))
		assert.Equal(t, []string{c.Connect(), c.Back()}, telegram.ButtonTexts(s.Telegram.Buttons(t)))
	})

	t.Run("unavailable Zvuk leaves the account unconnected", func(t *testing.T) {
		s := harness.New(t)
		c := s.Catalog(alice)
		s.Zvuk.AddAccount(harness.ZvukToken, true)
		s.Zvuk.SetDown(true)

		s.SendZvukToken(alice, harness.ZvukToken)

		assert.Contains(t, s.WindowText(), c.ProviderDown("zvuk"))
	})

	t.Run("services to import from are those with a collection", func(t *testing.T) {
		s := harness.New(t)
		c := s.Catalog(alice)

		s.Open(alice, s.Catalog(alice).ImportButton())

		assert.Equal(t, []string{c.ProviderButton("zvuk"), c.ImportsButton(), c.Back()}, telegram.ButtonTexts(s.Telegram.Buttons(t)))
	})

	t.Run("zvuk commands are gone", func(t *testing.T) {
		s := harness.New(t)
		s.Zvuk.AddAccount(harness.ZvukToken, true)

		commands := []*models.Update{
			s.TextMessage(alice, "/zvuk "+harness.ZvukToken),
			s.TextMessage(alice, "/zvuk_import"),
			s.TextMessage(alice, "/zvuk_off"),
		}
		for _, command := range commands {
			s.Send(command)
		}

		assertOnlyDeleted(t, s, commands...)
	})
}

func TestZvukReconnect(t *testing.T) {
	t.Parallel()

	t.Run("reconnected Zvuk keeps updating the same Navidrome playlist", func(t *testing.T) {
		s := harness.New(t)
		account := s.LinkNewAccount(alice)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.ImportZvuk(alice)
		s.WaitIngest()
		s.Navidrome.UntilPlaylist(t, account, "My Playlist", []string{"Listed/Song 1", "Liked/Song 1"})
		added := s.AddZvukAlbum("685", "Again", 1)

		s.OpenZvuk(alice, s.Catalog(alice).Disconnect())
		s.SendZvukToken(alice, harness.ZvukToken)
		s.Zvuk.UpdatePlaylist("810", func(p *zvuk.Playlist) { p.Tracks = append(p.Tracks, added...) })
		s.Sync()
		s.WaitIngest()

		s.Navidrome.UntilPlaylist(t, account, "My Playlist", []string{"Listed/Song 1", "Liked/Song 1", "Again/Song 1"})
		named := 0
		for _, p := range s.Navidrome.Playlists(t, account) {
			if p.Name == "My Playlist" {
				named++
			}
		}
		assert.Equal(t, 1, named)
	})
}
