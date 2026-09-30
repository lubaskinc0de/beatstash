package e2e

import (
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/telegram"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/zvuk"
)

const syncInterval = 100 * time.Millisecond

func TestZvukSync(t *testing.T) {
	t.Parallel()

	t.Run("new like is downloaded and starred", func(t *testing.T) {
		s := harness.New(t, harness.WithSyncInterval(syncInterval))
		account := s.LinkNewAccount(alice)
		s.ConnectZvuk(alice, harness.ZvukToken)
		collection := s.AddZvukCollection(harness.ZvukToken)
		s.ImportZvuk(alice)
		s.WaitIngest()

		fresh := s.AddZvukAlbum("670", "Fresh", 1)
		s.Zvuk.Update(harness.ZvukToken, func(a *zvuk.Account) { a.Liked = append(a.Liked, fresh...) })

		untilPersonalFile(t, s, alice, "Zvuk Band/Fresh (2020)/01 - Song 1.mp3")
		s.Navidrome.UntilStarred(t, account, []string{
			"Fresh/Song 1", "Liked/Song 1", "Liked/Song 2", "Liked/Song 3", "Liked/Song 4", "Liked/Song 5",
		})
		assert.Equal(t, 1, s.Zvuk.DownloadsOf(collection.Liked[0]))
	})

	t.Run("like of a track Zvuk stopped serving keeps its star", func(t *testing.T) {
		s := harness.New(t, harness.WithSyncInterval(syncInterval))
		account := s.LinkNewAccount(alice)
		s.ConnectZvuk(alice, harness.ZvukToken)
		collection := s.AddZvukCollection(harness.ZvukToken)
		s.ImportZvuk(alice)
		s.WaitIngest()
		s.Navidrome.UntilStarred(t, account, []string{
			"Liked/Song 1", "Liked/Song 2", "Liked/Song 3", "Liked/Song 4", "Liked/Song 5",
		})

		s.Zvuk.MakeUnavailable(collection.Liked[0])
		untilSynced(t, s, 2)

		assert.Len(t, s.Navidrome.Starred(t, account), 5)
	})

	t.Run("unliked track loses its star but stays in the library", func(t *testing.T) {
		s := harness.New(t, harness.WithSyncInterval(syncInterval))
		account := s.LinkNewAccount(alice)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.ImportZvuk(alice)
		s.WaitIngest()
		s.Navidrome.UntilStarred(t, account, []string{
			"Liked/Song 1", "Liked/Song 2", "Liked/Song 3", "Liked/Song 4", "Liked/Song 5",
		})

		s.Zvuk.Update(harness.ZvukToken, func(a *zvuk.Account) { a.Liked = a.Liked[1:] })

		s.Navidrome.UntilStarred(t, account, []string{"Liked/Song 2", "Liked/Song 3", "Liked/Song 4", "Liked/Song 5"})
		assert.Contains(t, s.PersonalFiles(alice), "Zvuk Band/Liked (2020)/01 - Song 1.mp3")
	})

	t.Run("track added to a playlist joins the Navidrome playlist", func(t *testing.T) {
		s := harness.New(t, harness.WithSyncInterval(syncInterval))
		account := s.LinkNewAccount(alice)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.ImportZvuk(alice)
		s.WaitIngest()
		s.Navidrome.UntilPlaylist(t, account, "My Playlist", []string{"Listed/Song 1", "Liked/Song 1"})

		added := s.AddZvukAlbum("680", "Added", 1)
		s.Zvuk.UpdatePlaylist("810", func(p *zvuk.Playlist) { p.Tracks = append(p.Tracks, added...) })

		s.Navidrome.UntilPlaylist(t, account, "My Playlist", []string{"Listed/Song 1", "Liked/Song 1", "Added/Song 1"})
	})

	t.Run("Sync waits for the running Import", func(t *testing.T) {
		s := harness.New(t, harness.WithSyncInterval(syncInterval))
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		held := s.Zvuk.HoldStreams(0)

		s.ImportZvuk(alice)
		<-held

		requests := s.Zvuk.RequestsOf("userCollection")
		require.Never(t, func() bool {
			return s.Zvuk.RequestsOf("userCollection") > requests
		}, 5*syncInterval, 20*time.Millisecond)
	})

	t.Run("failed Sync waits a whole interval before trying again", func(t *testing.T) {
		s := harness.New(t, harness.WithSyncInterval(time.Second))
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.ImportZvuk(alice)
		s.WaitIngest()

		s.Zvuk.SetDown(true)

		require.Never(t, func() bool {
			return s.Zvuk.RefusedRequests() > 3
		}, 2500*time.Millisecond, 50*time.Millisecond)
	})

	t.Run("disconnected Zvuk is not synced", func(t *testing.T) {
		s := harness.New(t, harness.WithSyncInterval(syncInterval))
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.ImportZvuk(alice)
		s.WaitIngest()

		s.OpenZvuk(alice, "🚫 Отключить")

		requests := s.Zvuk.RequestsOf("userCollection")
		require.Never(t, func() bool {
			return s.Zvuk.RequestsOf("userCollection") > requests
		}, 5*syncInterval, 20*time.Millisecond)
	})

	t.Run("anti-bot refusal does not revoke the token", func(t *testing.T) {
		s := harness.New(t, harness.WithSyncInterval(syncInterval))
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.ImportZvuk(alice)
		s.WaitIngest()

		s.Zvuk.SetGuarded(true)

		require.Eventually(t, func() bool {
			return s.Zvuk.RefusedRequests() >= 2
		}, 10*time.Second, 20*time.Millisecond)
		assert.Empty(t, s.SentMessagesContaining(alice, tokenRejected))
	})

	t.Run("revoked token pauses Sync and tells the user once", func(t *testing.T) {
		s := harness.New(t, harness.WithSyncInterval(syncInterval))
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.ImportZvuk(alice)
		s.WaitIngest()

		s.Zvuk.Revoke(harness.ZvukToken)

		untilTokenRejected(t, s)
		requests := s.Zvuk.RequestsOf("userCollection")
		require.Never(t, func() bool {
			return s.Zvuk.RequestsOf("userCollection") > requests
		}, 5*syncInterval, 20*time.Millisecond)
		assert.Len(t, s.SentMessagesContaining(alice, tokenRejected), 1)
	})

	t.Run("two instances tell of a revoked token once", func(t *testing.T) {
		s := harness.New(t, harness.WithSyncInterval(syncInterval))
		s.StartReplica()
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.ImportZvuk(alice)
		s.WaitIngest()
		notice := s.Telegram.Hold("sendMessage")
		sent := len(s.Telegram.CallsTo("sendMessage"))

		s.Zvuk.Revoke(harness.ZvukToken)
		<-notice.Arrived()
		assert.False(t, s.Telegram.WaitCalls("sendMessage", sent+1, anotherRound), "another instance acted meanwhile")
		notice.Release()

		untilTokenRejected(t, s)
		assert.Len(t, s.SentMessagesContaining(alice, tokenRejected), 1)
	})

	t.Run("token revoked again after reconnect is told of again", func(t *testing.T) {
		s := harness.New(t, harness.WithSyncInterval(syncInterval))
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.ImportZvuk(alice)
		s.WaitIngest()
		s.Zvuk.Revoke(harness.ZvukToken)
		untilTokenRejected(t, s)

		s.ConnectZvuk(alice, harness.ZvukToken)
		s.Zvuk.Revoke(harness.ZvukToken)

		require.Eventually(t, func() bool {
			return len(s.SentMessagesContaining(alice, tokenRejected)) == 2
		}, 10*time.Second, 20*time.Millisecond)
	})

	t.Run("revoked token notice leads to the Zvuk screen", func(t *testing.T) {
		s := harness.New(t, harness.WithSyncInterval(syncInterval))
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.ImportZvuk(alice)
		s.WaitIngest()
		s.Zvuk.Revoke(harness.ZvukToken)
		untilTokenRejected(t, s)

		notice := s.Telegram.MessageWith(s.Button("🔌 Подключить заново"))

		s.Go(alice, "🔌 Подключить заново")

		assert.NotContains(t, s.Telegram.EditedMessages(), strconv.Itoa(notice))
		assert.Contains(t, s.WindowText(), "больше не принимает токен")
		assert.Equal(t, []string{"🔌 Подключить", "← Назад"}, telegram.ButtonTexts(s.Telegram.Buttons(t)))
	})

	t.Run("reconnected Zvuk resumes Sync", func(t *testing.T) {
		s := harness.New(t, harness.WithSyncInterval(syncInterval))
		s.ConnectZvuk(alice, harness.ZvukToken)
		collection := s.AddZvukCollection(harness.ZvukToken)
		s.ImportZvuk(alice)
		s.WaitIngest()
		s.Zvuk.Revoke(harness.ZvukToken)
		untilTokenRejected(t, s)
		fresh := s.AddZvukAlbum("690", "Back", 1)

		s.ConnectZvuk(alice, harness.ZvukToken)
		s.Zvuk.Update(harness.ZvukToken, func(a *zvuk.Account) {
			a.Liked = append(slices.Clone(collection.Liked), fresh...)
		})

		untilPersonalFile(t, s, alice, "Zvuk Band/Back (2020)/01 - Song 1.mp3")
	})
}

const tokenRejected = "Звук перестал принимать токен" //nolint:gosec // G101: message text, not a credential

func untilTokenRejected(t *testing.T, s *harness.Scenario) {
	t.Helper()

	require.Eventually(t, func() bool {
		return len(s.SentMessagesContaining(alice, tokenRejected)) > 0
	}, 10*time.Second, 20*time.Millisecond, "the user never heard of the rejected token")
}

// untilSynced waits for n more Syncs; the Mirror of all but the last one
// has run by then.
func untilSynced(t *testing.T, s *harness.Scenario, n int) {
	t.Helper()

	target := s.Zvuk.RequestsOf("userCollection") + n
	require.Eventually(t, func() bool {
		return s.Zvuk.RequestsOf("userCollection") >= target
	}, 10*time.Second, 20*time.Millisecond, "Sync never ran")
}

func untilPersonalFile(t *testing.T, s *harness.Scenario, user harness.User, rel string) {
	t.Helper()

	require.Eventually(t, func() bool {
		return slices.Contains(s.PersonalFiles(user), rel)
	}, 10*time.Second, 20*time.Millisecond, "%s never appeared", rel)
}
