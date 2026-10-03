package e2e

import (
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/lubaskinc0de/beatstash/e2e/harness"
	"github.com/lubaskinc0de/beatstash/e2e/harness/telegram"
	"github.com/lubaskinc0de/beatstash/e2e/harness/zvuk"
)

func TestZvukSync(t *testing.T) {
	t.Parallel()

	t.Run("new like is downloaded and starred", func(t *testing.T) {
		s := harness.New(t)
		account := s.LinkNewAccount(alice)
		s.ConnectZvuk(alice, harness.ZvukToken)
		collection := s.AddZvukCollection(harness.ZvukToken)
		s.ImportZvuk(alice)
		s.WaitIngest()
		fresh := s.AddZvukAlbum("670", "Fresh", 1)
		s.Zvuk.Update(harness.ZvukToken, func(a *zvuk.Account) { a.Liked = append(a.Liked, fresh...) })

		s.Sync()
		s.WaitIngest()

		assert.Contains(t, s.PersonalFiles(alice), "Zvuk Band/Fresh (2020)/01 - Song 1.mp3")
		s.Navidrome.UntilStarred(t, account, []string{
			"Fresh/Song 1", "Liked/Song 1", "Liked/Song 2", "Liked/Song 3", "Liked/Song 4", "Liked/Song 5",
		})
		assert.Equal(t, 1, s.Zvuk.DownloadsOf(collection.Liked[0]))
	})

	t.Run("like of a track Zvuk stopped serving keeps its star", func(t *testing.T) {
		s := harness.New(t)
		account := s.LinkNewAccount(alice)
		s.ConnectZvuk(alice, harness.ZvukToken)
		collection := s.AddZvukCollection(harness.ZvukToken)
		s.ImportZvuk(alice)
		s.WaitIngest()
		s.Navidrome.UntilStarred(t, account, []string{
			"Liked/Song 1", "Liked/Song 2", "Liked/Song 3", "Liked/Song 4", "Liked/Song 5",
		})

		s.Zvuk.MakeUnavailable(collection.Liked[0])
		s.Sync()

		assert.Len(t, s.Navidrome.Starred(t, account), 5)
	})

	t.Run("unliked track loses its star but stays in the library", func(t *testing.T) {
		s := harness.New(t)
		account := s.LinkNewAccount(alice)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.ImportZvuk(alice)
		s.WaitIngest()
		s.Navidrome.UntilStarred(t, account, []string{
			"Liked/Song 1", "Liked/Song 2", "Liked/Song 3", "Liked/Song 4", "Liked/Song 5",
		})

		s.Zvuk.Update(harness.ZvukToken, func(a *zvuk.Account) { a.Liked = a.Liked[1:] })
		s.Sync()

		s.Navidrome.UntilStarred(t, account, []string{"Liked/Song 2", "Liked/Song 3", "Liked/Song 4", "Liked/Song 5"})
		assert.Contains(t, s.PersonalFiles(alice), "Zvuk Band/Liked (2020)/01 - Song 1.mp3")
	})

	t.Run("track added to a playlist joins the Navidrome playlist", func(t *testing.T) {
		s := harness.New(t)
		account := s.LinkNewAccount(alice)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.ImportZvuk(alice)
		s.WaitIngest()
		s.Navidrome.UntilPlaylist(t, account, "My Playlist", []string{"Listed/Song 1", "Liked/Song 1"})

		added := s.AddZvukAlbum("680", "Added", 1)
		s.Zvuk.UpdatePlaylist("810", func(p *zvuk.Playlist) { p.Tracks = append(p.Tracks, added...) })
		s.Sync()
		s.WaitIngest()

		s.Navidrome.UntilPlaylist(t, account, "My Playlist", []string{"Listed/Song 1", "Liked/Song 1", "Added/Song 1"})
	})

	t.Run("Sync waits for the running Import", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		held := s.Zvuk.HoldStreams(0)

		s.ImportZvuk(alice)
		<-held

		requests := s.Zvuk.RequestsOf("userCollection")
		s.Sync()

		assert.Equal(t, requests, s.Zvuk.RequestsOf("userCollection"))
	})

	t.Run("failed Sync waits a whole interval before trying again", func(t *testing.T) {
		const interval = time.Hour
		s := harness.New(t, harness.WithSyncInterval(interval))
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.ImportZvuk(alice)
		s.WaitIngest()
		s.Zvuk.SetDown(true)
		s.Sync()
		refused := s.Zvuk.RefusedRequests()

		s.Clock.Advance(interval - time.Minute)
		s.RunSync()
		assert.Equal(t, refused, s.Zvuk.RefusedRequests(), "Sync came before the interval")

		s.Clock.Advance(time.Minute)
		s.RunSync()
		assert.Greater(t, s.Zvuk.RefusedRequests(), refused, "Sync did not come after the interval")
	})

	t.Run("disconnected Zvuk is not synced", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.ImportZvuk(alice)
		s.WaitIngest()

		s.OpenZvuk(alice, s.Catalog(alice).Disconnect())

		requests := s.Zvuk.RequestsOf("userCollection")
		s.Sync()

		assert.Equal(t, requests, s.Zvuk.RequestsOf("userCollection"))
	})

	t.Run("anti-bot refusal does not revoke the token", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.ImportZvuk(alice)
		s.WaitIngest()

		s.Zvuk.SetGuarded(true)

		s.Sync()
		refused := s.Zvuk.RefusedRequests()
		s.Sync()
		s.WaitIngest()

		assert.Greater(t, s.Zvuk.RefusedRequests(), refused, "the token was taken for revoked")
		assert.Empty(t, s.SentMessagesContaining(alice, tokenRejected))
	})

	t.Run("revoked token pauses Sync and tells the user once", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.ImportZvuk(alice)
		s.WaitIngest()

		s.Zvuk.Revoke(harness.ZvukToken)

		s.Sync()
		requests := s.Zvuk.RequestsOf("userCollection")
		s.Sync()
		s.WaitIngest()

		assert.Equal(t, requests, s.Zvuk.RequestsOf("userCollection"))
		assert.Len(t, s.SentMessagesContaining(alice, tokenRejected), 1)
	})

	t.Run("two instances tell of a revoked token once", func(t *testing.T) {
		s := harness.New(t)
		s.StartReplica()
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.ImportZvuk(alice)
		s.WaitIngest()
		notice := s.Telegram.Hold("sendMessage")
		sent := len(s.Telegram.CallsTo("sendMessage"))

		s.Zvuk.Revoke(harness.ZvukToken)
		s.Sync()
		<-notice.Arrived()
		s.PollAny()
		assert.Len(t, s.Telegram.CallsTo("sendMessage"), sent, "another instance acted meanwhile")
		notice.Release()
		s.WaitIngest()

		assert.Len(t, s.SentMessagesContaining(alice, tokenRejected), 1)
	})

	t.Run("token revoked again after reconnect is told of again", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.ImportZvuk(alice)
		s.WaitIngest()
		s.Zvuk.Revoke(harness.ZvukToken)
		s.Sync()
		s.WaitIngest()

		s.ConnectZvuk(alice, harness.ZvukToken)
		s.Zvuk.Revoke(harness.ZvukToken)
		s.Sync()
		s.WaitIngest()

		assert.Len(t, s.SentMessagesContaining(alice, tokenRejected), 2)
	})

	t.Run("revoked token notice leads to the Zvuk screen", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.ImportZvuk(alice)
		s.WaitIngest()
		s.Zvuk.Revoke(harness.ZvukToken)
		s.Sync()
		s.WaitIngest()

		notice := s.Telegram.MessageWith(s.Button(s.Catalog(alice).Reconnect()))

		s.Go(alice, s.Catalog(alice).Reconnect())

		assert.NotContains(t, s.Telegram.EditedMessages(), strconv.Itoa(notice))
		c := s.Catalog(alice)
		assert.Contains(t, s.WindowText(), c.Provider("zvuk", "token_rejected"))
		assert.Equal(t, []string{c.Connect(), c.Back()}, telegram.ButtonTexts(s.Telegram.Buttons(t)))
	})

	t.Run("reconnected Zvuk resumes Sync", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		collection := s.AddZvukCollection(harness.ZvukToken)
		s.ImportZvuk(alice)
		s.WaitIngest()
		s.Zvuk.Revoke(harness.ZvukToken)
		s.Sync()
		s.WaitIngest()
		fresh := s.AddZvukAlbum("690", "Back", 1)

		s.ConnectZvuk(alice, harness.ZvukToken)
		s.Zvuk.Update(harness.ZvukToken, func(a *zvuk.Account) {
			a.Liked = append(slices.Clone(collection.Liked), fresh...)
		})
		s.Sync()
		s.WaitIngest()

		assert.Contains(t, s.PersonalFiles(alice), "Zvuk Band/Back (2020)/01 - Song 1.mp3")
	})
}

const tokenRejected = "Звук перестал принимать токен" //nolint:gosec // G101: message text, not a credential
