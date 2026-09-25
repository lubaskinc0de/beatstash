package e2e

import (
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	app "github.com/lubaskinc0de/navidrome-tg/internal/main"
)

const syncInterval = 100 * time.Millisecond

func withSyncInterval(d time.Duration) scenarioOption {
	return func(c *app.Config) { c.SyncInterval = d }
}

func TestZvukSync(t *testing.T) {
	t.Parallel()

	t.Run("new like is downloaded and starred", func(t *testing.T) {
		s := newScenario(t, withSyncInterval(syncInterval))
		account := s.linkNewAccount(alice)
		s.connectZvuk(alice, zvukToken)
		collection := s.addZvukCollection(zvukToken)
		s.importZvuk(alice)
		s.waitIngest()

		fresh := s.addZvukAlbum("670", "Fresh", 1)
		s.zvuk.update(zvukToken, func(a *zvukAccount) { a.liked = append(a.liked, fresh...) })

		s.untilPersonalFile(alice, "Zvuk Band/Fresh (2020)/01 - Song 1.mp3")
		env.navidrome.untilStarred(t, account, []string{
			"Fresh/Song 1", "Liked/Song 1", "Liked/Song 2", "Liked/Song 3", "Liked/Song 4", "Liked/Song 5",
		})
		assert.Equal(t, 1, s.zvuk.downloadsOf(collection.liked[0]))
	})

	t.Run("like of a track Zvuk stopped serving keeps its star", func(t *testing.T) {
		s := newScenario(t, withSyncInterval(syncInterval))
		account := s.linkNewAccount(alice)
		s.connectZvuk(alice, zvukToken)
		collection := s.addZvukCollection(zvukToken)
		s.importZvuk(alice)
		s.waitIngest()
		env.navidrome.untilStarred(t, account, []string{
			"Liked/Song 1", "Liked/Song 2", "Liked/Song 3", "Liked/Song 4", "Liked/Song 5",
		})

		s.zvuk.makeUnavailable(collection.liked[0])
		s.untilSynced(2)

		assert.Len(t, env.navidrome.starred(t, account), 5)
	})

	t.Run("unliked track loses its star but stays in the library", func(t *testing.T) {
		s := newScenario(t, withSyncInterval(syncInterval))
		account := s.linkNewAccount(alice)
		s.connectZvuk(alice, zvukToken)
		s.addZvukCollection(zvukToken)
		s.importZvuk(alice)
		s.waitIngest()
		env.navidrome.untilStarred(t, account, []string{
			"Liked/Song 1", "Liked/Song 2", "Liked/Song 3", "Liked/Song 4", "Liked/Song 5",
		})

		s.zvuk.update(zvukToken, func(a *zvukAccount) { a.liked = a.liked[1:] })

		env.navidrome.untilStarred(t, account, []string{"Liked/Song 2", "Liked/Song 3", "Liked/Song 4", "Liked/Song 5"})
		assert.Contains(t, s.personalFiles(alice), "Zvuk Band/Liked (2020)/01 - Song 1.mp3")
	})

	t.Run("track added to a playlist joins the Navidrome playlist", func(t *testing.T) {
		s := newScenario(t, withSyncInterval(syncInterval))
		account := s.linkNewAccount(alice)
		s.connectZvuk(alice, zvukToken)
		s.addZvukCollection(zvukToken)
		s.importZvuk(alice)
		s.waitIngest()
		env.navidrome.untilPlaylist(t, account, "My Playlist", []string{"Listed/Song 1", "Liked/Song 1"})

		added := s.addZvukAlbum("680", "Added", 1)
		s.zvuk.updatePlaylist("810", func(p *zvukPlaylist) { p.Tracks = append(p.Tracks, added...) })

		env.navidrome.untilPlaylist(t, account, "My Playlist", []string{"Listed/Song 1", "Liked/Song 1", "Added/Song 1"})
	})

	t.Run("Sync waits for the running Import", func(t *testing.T) {
		s := newScenario(t, withSyncInterval(syncInterval))
		s.connectZvuk(alice, zvukToken)
		s.addZvukCollection(zvukToken)
		held := s.zvuk.holdStreams(0)

		s.importZvuk(alice)
		<-held

		requests := s.zvuk.requestsOf("userCollection")
		require.Never(t, func() bool {
			return s.zvuk.requestsOf("userCollection") > requests
		}, 5*syncInterval, 20*time.Millisecond)
	})

	t.Run("failed Sync waits a whole interval before trying again", func(t *testing.T) {
		s := newScenario(t, withSyncInterval(time.Second))
		s.connectZvuk(alice, zvukToken)
		s.addZvukCollection(zvukToken)
		s.importZvuk(alice)
		s.waitIngest()

		s.zvuk.setDown(true)

		require.Never(t, func() bool {
			return s.zvuk.refusedRequests() > 3
		}, 2500*time.Millisecond, 50*time.Millisecond)
	})

	t.Run("disconnected Zvuk is not synced", func(t *testing.T) {
		s := newScenario(t, withSyncInterval(syncInterval))
		s.connectZvuk(alice, zvukToken)
		s.addZvukCollection(zvukToken)
		s.importZvuk(alice)
		s.waitIngest()

		s.send(s.textMessage(alice, "/zvuk_off"))

		requests := s.zvuk.requestsOf("userCollection")
		require.Never(t, func() bool {
			return s.zvuk.requestsOf("userCollection") > requests
		}, 5*syncInterval, 20*time.Millisecond)
	})

	t.Run("anti-bot refusal does not revoke the token", func(t *testing.T) {
		s := newScenario(t, withSyncInterval(syncInterval))
		s.connectZvuk(alice, zvukToken)
		s.addZvukCollection(zvukToken)
		s.importZvuk(alice)
		s.waitIngest()

		s.zvuk.setGuarded(true)

		require.Eventually(t, func() bool {
			return s.zvuk.refusedRequests() >= 2
		}, 10*time.Second, 20*time.Millisecond)
		assert.Empty(t, s.sentMessagesContaining(tokenRejected))
	})

	t.Run("revoked token pauses Sync and tells the user once", func(t *testing.T) {
		s := newScenario(t, withSyncInterval(syncInterval))
		s.connectZvuk(alice, zvukToken)
		s.addZvukCollection(zvukToken)
		s.importZvuk(alice)
		s.waitIngest()

		s.zvuk.revoke(zvukToken)

		s.untilTokenRejected()
		requests := s.zvuk.requestsOf("userCollection")
		require.Never(t, func() bool {
			return s.zvuk.requestsOf("userCollection") > requests
		}, 5*syncInterval, 20*time.Millisecond)
		assert.Len(t, s.sentMessagesContaining(tokenRejected), 1)
	})

	t.Run("reconnected Zvuk resumes Sync", func(t *testing.T) {
		s := newScenario(t, withSyncInterval(syncInterval))
		s.connectZvuk(alice, zvukToken)
		collection := s.addZvukCollection(zvukToken)
		s.importZvuk(alice)
		s.waitIngest()
		s.zvuk.revoke(zvukToken)
		s.untilTokenRejected()
		fresh := s.addZvukAlbum("690", "Back", 1)

		s.connectZvuk(alice, zvukToken)
		s.zvuk.update(zvukToken, func(a *zvukAccount) {
			a.liked = append(slices.Clone(collection.liked), fresh...)
		})

		s.untilPersonalFile(alice, "Zvuk Band/Back (2020)/01 - Song 1.mp3")
	})
}

func TestZvukReconnect(t *testing.T) {
	t.Parallel()

	t.Run("reconnected Zvuk keeps updating the same Navidrome playlist", func(t *testing.T) {
		s := newScenario(t, withSyncInterval(syncInterval))
		account := s.linkNewAccount(alice)
		s.connectZvuk(alice, zvukToken)
		s.addZvukCollection(zvukToken)
		s.importZvuk(alice)
		s.waitIngest()
		env.navidrome.untilPlaylist(t, account, "My Playlist", []string{"Listed/Song 1", "Liked/Song 1"})
		added := s.addZvukAlbum("685", "Again", 1)

		s.send(s.textMessage(alice, "/zvuk_off"))
		s.send(s.textMessage(alice, "/zvuk "+zvukToken))
		s.zvuk.updatePlaylist("810", func(p *zvukPlaylist) { p.Tracks = append(p.Tracks, added...) })

		env.navidrome.untilPlaylist(t, account, "My Playlist", []string{"Listed/Song 1", "Liked/Song 1", "Again/Song 1"})
		named := 0
		for _, p := range env.navidrome.playlists(t, account) {
			if p.Name == "My Playlist" {
				named++
			}
		}
		assert.Equal(t, 1, named)
	})
}

const tokenRejected = "Звук перестал принимать токен"

func (s *scenario) untilTokenRejected() {
	s.t.Helper()

	require.Eventually(s.t, func() bool {
		return len(s.sentMessagesContaining(tokenRejected)) > 0
	}, 10*time.Second, 20*time.Millisecond, "the user never heard of the rejected token")
}

// untilSynced waits for n more Syncs; the Mirror of all but the last one
// has run by then.
func (s *scenario) untilSynced(n int) {
	s.t.Helper()

	target := s.zvuk.requestsOf("userCollection") + n
	require.Eventually(s.t, func() bool {
		return s.zvuk.requestsOf("userCollection") >= target
	}, 10*time.Second, 20*time.Millisecond, "Sync never ran")
}

func (s *scenario) untilPersonalFile(user telegramUser, rel string) {
	s.t.Helper()

	require.Eventually(s.t, func() bool {
		return slices.Contains(s.personalFiles(user), rel)
	}, 10*time.Second, 20*time.Millisecond, "%s never appeared", rel)
}
