package e2e

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPersonalLibrary(t *testing.T) {
	t.Run("upload stays private to its uploader", func(t *testing.T) {
		s := newScenario(t)
		aliceAccount := s.linkNewAccount(alice)
		bobAccount := s.linkNewAccount(bob)

		s.send(s.audioMessage(alice, s.uploadAudio("track.mp3")))
		s.waitIngest()

		assert.Equal(t, []string{"Fixture Artist/Fixture Album/01 - Fixture Song.mp3"}, s.personalFiles(alice))
		env.navidrome.indexedTrack(t, aliceAccount, s.library, fixtureTitle)
		assert.Empty(t, env.navidrome.searchFor(t, bobAccount, s.library, fixtureTitle))
	})

	t.Run("same track from two users is kept by each", func(t *testing.T) {
		s := newScenario(t)
		mp3 := makeAudio(t, "song.mp3", audioSpec{Tags: songTags})

		s.send(s.audioMessage(alice, s.uploadAudioFile(mp3)))
		s.waitIngest()
		s.send(s.audioMessage(bob, s.uploadAudioFile(mp3)))
		s.waitIngest()

		assert.Equal(t, []string{"Artist/Album/01 - Dup Song.mp3"}, s.personalFiles(alice))
		assert.Equal(t, []string{"Artist/Album/01 - Dup Song.mp3"}, s.personalFiles(bob))
		assert.Empty(t, s.botAPI.Replies(t))
	})

	t.Run("each user has their own Inbox", func(t *testing.T) {
		s := newScenario(t)
		untagged := makeAudio(t, "noise.mp3", audioSpec{})

		s.send(s.audioMessage(alice, s.uploadAudioFile(untagged)))
		s.send(s.audioMessage(bob, s.uploadAudioFile(untagged)))
		s.waitIngest()

		assert.Equal(t, []string{"Inbox/noise.mp3"}, s.personalFiles(alice))
		assert.Equal(t, []string{"Inbox/noise.mp3"}, s.personalFiles(bob))
	})

	t.Run("newcomer's account sees only their own and the Shared Library", func(t *testing.T) {
		s := newScenario(t)
		carol := newcomer("carol")

		s.send(s.textMessage(carol, "/start "+s.invite()))

		account := issuedAccount(t, s)
		assert.Equal(t, []string{
			s.navidromePath("shared"),
			s.navidromePath(personalDir(carol)),
		}, env.navidrome.libraries(t, account))
	})
}

func TestNavidromeDefaults(t *testing.T) {
	t.Run("account made in Navidrome sees the Shared Library but no Personal one", func(t *testing.T) {
		s := newScenario(t)

		account := env.navidrome.createAccount(t, "dave")

		libraries := env.navidrome.libraries(t, account)
		assert.Contains(t, libraries, s.navidromePath("shared"))
		for _, user := range []telegramUser{adminUser, alice, bob} {
			assert.NotContains(t, libraries, s.navidromePath(personalDir(user)))
		}
	})
}

func TestNavidromeLibraryGone(t *testing.T) {
	t.Run("library deleted in Navidrome is found again by its path", func(t *testing.T) {
		s := newScenario(t)
		account := s.linkNewAccount(alice)
		env.navidrome.deleteLibrary(t, env.navidrome.libraryAt(t, s.navidromePath("shared")))
		id := env.navidrome.createLibrary(t, uniqueLogin("moved"), s.navidromePath("shared"))

		s.restart()

		assert.Equal(t, []string{
			s.navidromePath("shared"),
			s.navidromePath(personalDir(alice)),
		}, env.navidrome.libraries(t, account))
		assert.Equal(t, id, env.navidrome.libraryAt(t, s.navidromePath("shared")))
	})

	t.Run("library deleted in Navidrome is created again", func(t *testing.T) {
		s := newScenario(t)
		account := s.linkNewAccount(alice)
		env.navidrome.deleteLibrary(t, env.navidrome.libraryAt(t, s.navidromePath("shared")))

		s.restart()

		assert.Equal(t, []string{
			s.navidromePath("shared"),
			s.navidromePath(personalDir(alice)),
		}, env.navidrome.libraries(t, account))
	})
}

func TestLinkAfterMove(t *testing.T) {
	t.Run("account linked to another user is refused", func(t *testing.T) {
		s := newScenario(t)
		account := s.linkNewAccount(alice)

		s.link(bob, account)

		assert.Contains(t, lastReply(t, s).Text, "Этот аккаунт Navidrome уже привязан к другому пользователю")
		assert.Equal(t, []string{
			s.navidromePath("shared"),
			s.navidromePath(personalDir(alice)),
		}, env.navidrome.libraries(t, account))
	})

	t.Run("login case does not get around another user's link", func(t *testing.T) {
		s := newScenario(t)
		account := s.linkNewAccount(alice)
		shouted := navidromeAccount{Login: strings.ToUpper(account.Login), Password: account.Password}

		s.link(bob, shouted)

		assert.Contains(t, lastReply(t, s).Text, "уже привязан к другому пользователю")
		assert.Contains(t, env.navidrome.libraries(t, account), s.navidromePath(personalDir(alice)))
	})

	t.Run("relinking one's own account works", func(t *testing.T) {
		s := newScenario(t)
		account := s.linkNewAccount(alice)

		s.link(alice, account)

		assert.Contains(t, lastReply(t, s).Text, "привязан. Теперь")
	})

	t.Run("link warns and hides others' Personal Libraries", func(t *testing.T) {
		s := newScenario(t)
		s.send(s.audioMessage(bob, s.uploadAudio("track.mp3")))
		s.waitIngest()
		account := env.navidrome.createAccount(t, "alice")
		env.navidrome.grantAllLibraries(t, account)
		bobAccount := s.linkNewAccount(bob)
		env.navidrome.indexedTrack(t, bobAccount, s.library, fixtureTitle)

		s.link(alice, account)

		assert.Contains(t, lastReply(t, s).Text, "только вашу личную библиотеку и общую")
		assert.Empty(t, env.navidrome.searchFor(t, account, s.library, fixtureTitle))
	})
}

func (s *scenario) linkNewAccount(user telegramUser) navidromeAccount {
	s.t.Helper()

	account := env.navidrome.createAccount(s.t, user.Username)
	s.link(user, account)
	require.Contains(s.t, lastReply(s.t, s).Text, "привязан")
	return account
}

func (s *scenario) navidromePath(rel string) string {
	return filepath.Join(s.config.NavidromeMusicDir, rel)
}
