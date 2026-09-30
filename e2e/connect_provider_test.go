package e2e

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/telegram"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/zvuk"
)

func TestZvukAccount(t *testing.T) {
	t.Parallel()

	t.Run("valid token connects Zvuk", func(t *testing.T) {
		s := harness.New(t)
		s.Zvuk.AddAccount(harness.ZvukToken, true)

		msg := s.SendZvukToken(alice, harness.ZvukToken)

		assert.Contains(t, s.Telegram.DeletedMessages(), strconv.Itoa(msg.Message.ID))
		assert.Contains(t, s.WindowText(), "Звук подключён")
		assert.Contains(t, s.WindowText(), "Статус: подключён")
	})

	t.Run("invalid token is rejected and leaves Zvuk unconnected", func(t *testing.T) {
		s := harness.New(t)

		msg := s.SendZvukToken(alice, "wrong-token")
		rejected := s.WindowText()
		s.OpenZvuk(alice)

		assert.Contains(t, s.Telegram.DeletedMessages(), strconv.Itoa(msg.Message.ID))
		assert.Contains(t, rejected, "Звук не принял токен")
		assert.Contains(t, s.WindowText(), "Статус: не подключён")
	})

	t.Run("account without subscription is rejected and stays unconnected", func(t *testing.T) {
		s := harness.New(t)
		s.Zvuk.AddAccount(harness.ZvukToken, false)

		s.SendZvukToken(alice, harness.ZvukToken)
		rejected := s.WindowText()
		s.OpenZvuk(alice)

		assert.Contains(t, rejected, "нет подписки")
		assert.Contains(t, s.WindowText(), "Статус: не подключён")
	})

	t.Run("user disconnects Zvuk", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)

		s.OpenZvuk(alice, "🚫 Отключить")

		assert.Contains(t, s.WindowText(), "Звук отключён")
		assert.Contains(t, s.WindowText(), "Статус: не подключён")
		assert.Equal(t, []string{"🔌 Подключить", "← Назад"}, telegram.ButtonTexts(s.Telegram.Buttons(t)))
	})

	t.Run("unavailable Zvuk leaves the account unconnected", func(t *testing.T) {
		s := harness.New(t)
		s.Zvuk.AddAccount(harness.ZvukToken, true)
		s.Zvuk.SetDown(true)

		s.SendZvukToken(alice, harness.ZvukToken)

		assert.Contains(t, s.WindowText(), "Звук недоступен")
	})

	t.Run("services to import from are those with a collection", func(t *testing.T) {
		s := harness.New(t)

		s.Open(alice, "📥 Импорт из музыкального сервиса")

		assert.Equal(t, []string{"🟣 Звук", "📊 Импорты", "← Назад"}, telegram.ButtonTexts(s.Telegram.Buttons(t)))
	})

	t.Run("zvuk commands are gone", func(t *testing.T) {
		s := harness.New(t)
		s.Zvuk.AddAccount(harness.ZvukToken, true)

		s.Send(s.TextMessage(alice, "/zvuk "+harness.ZvukToken))
		s.Send(s.TextMessage(alice, "/zvuk_import"))
		s.Send(s.TextMessage(alice, "/zvuk_off"))

		assert.Empty(t, s.Telegram.AllCalls())
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

		s.OpenZvuk(alice, "🚫 Отключить")
		s.SendZvukToken(alice, harness.ZvukToken)
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
