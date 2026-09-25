package e2e

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/zvuk"
)

func TestZvukAccount(t *testing.T) {
	t.Parallel()

	t.Run("valid token connects Zvuk", func(t *testing.T) {
		s := harness.New(t)
		s.Zvuk.AddAccount(harness.ZvukToken, true)
		msg := s.TextMessage(alice, "/zvuk "+harness.ZvukToken)

		s.Send(msg)

		assert.Contains(t, s.Telegram.DeletedMessages(), strconv.Itoa(msg.Message.ID))
		assert.Contains(t, s.LastReply().Text, "Звук подключён")
	})

	t.Run("invalid token is rejected", func(t *testing.T) {
		s := harness.New(t)
		msg := s.TextMessage(alice, "/zvuk wrong-token")

		s.Send(msg)

		assert.Contains(t, s.Telegram.DeletedMessages(), strconv.Itoa(msg.Message.ID))
		assert.Contains(t, s.LastReply().Text, "Звук не принял токен")
	})

	t.Run("rejected token leaves Zvuk unconnected", func(t *testing.T) {
		s := harness.New(t)
		s.Send(s.TextMessage(alice, "/zvuk wrong-token"))

		s.Send(s.TextMessage(alice, "/zvuk_import"))

		assert.Contains(t, s.LastReply().Text, "Подключите Звук")
	})

	t.Run("account without subscription is rejected", func(t *testing.T) {
		s := harness.New(t)
		s.Zvuk.AddAccount(harness.ZvukToken, false)

		s.Send(s.TextMessage(alice, "/zvuk "+harness.ZvukToken))

		assert.Contains(t, s.LastReply().Text, "нет подписки")
	})

	t.Run("account without subscription stays unconnected", func(t *testing.T) {
		s := harness.New(t)
		s.Zvuk.AddAccount(harness.ZvukToken, false)
		s.Send(s.TextMessage(alice, "/zvuk "+harness.ZvukToken))

		s.Send(s.TextMessage(alice, "/zvuk_import"))

		assert.Contains(t, s.LastReply().Text, "Подключите Звук")
	})

	t.Run("user disconnects Zvuk", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)

		s.Send(s.TextMessage(alice, "/zvuk_off"))

		assert.Contains(t, s.LastReply().Text, "Звук отключён")
	})

	t.Run("disconnected account asks to connect again", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.Send(s.TextMessage(alice, "/zvuk_off"))

		s.Send(s.TextMessage(alice, "/zvuk_import"))

		assert.Contains(t, s.LastReply().Text, "Подключите Звук")
	})

	t.Run("unavailable Zvuk leaves the account unconnected", func(t *testing.T) {
		s := harness.New(t)
		s.Zvuk.AddAccount(harness.ZvukToken, true)
		s.Zvuk.SetDown(true)

		s.Send(s.TextMessage(alice, "/zvuk "+harness.ZvukToken))

		assert.Contains(t, s.LastReply().Text, "Звук недоступен")
	})
}

func TestZvukReconnect(t *testing.T) {
	t.Parallel()

	t.Run("reconnected Zvuk keeps updating the same Navidrome playlist", func(t *testing.T) {
		s := harness.New(t, harness.WithSyncInterval(syncInterval))
		account := s.LinkNewAccount(alice)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.ImportZvuk(alice)
		s.WaitIngest()
		s.Navidrome.UntilPlaylist(t, account, "My Playlist", []string{"Listed/Song 1", "Liked/Song 1"})
		added := s.AddZvukAlbum("685", "Again", 1)

		s.Send(s.TextMessage(alice, "/zvuk_off"))
		s.Send(s.TextMessage(alice, "/zvuk "+harness.ZvukToken))
		s.Zvuk.UpdatePlaylist("810", func(p *zvuk.Playlist) { p.Tracks = append(p.Tracks, added...) })

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
