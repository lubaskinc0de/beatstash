package e2e

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/audiofile"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/navidrome"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/telegram"
)

func TestNavidromeAccount(t *testing.T) {
	t.Parallel()

	t.Run("each user sees their own np", func(t *testing.T) {
		s := harness.New(t)
		aliceAccount := s.Navidrome.CreateAccount(t, "alice")
		bobAccount := s.Navidrome.CreateAccount(t, "bob")
		s.Link(alice, aliceAccount)
		s.Link(bob, bobAccount)
		aliceAudio := s.UploadAudio("track.mp3")
		bobAudio := s.UploadAudioFile(audiofile.Generate(t, "other.mp3", audiofile.Spec{Tags: map[string]string{
			"artist": "Other Artist", "album": "Other Album", "title": "Other Song",
		}}))
		s.Send(s.AudioMessage(alice, aliceAudio))
		s.Send(s.AudioMessage(bob, bobAudio))
		s.Navidrome.StartPlaying(t, aliceAccount, s.Navidrome.IndexedTrack(t, aliceAccount, s.Library, audiofile.FixtureTitle).ID)
		s.Navidrome.StartPlaying(t, bobAccount, s.Navidrome.IndexedTrack(t, bobAccount, s.Library, "Other Song").ID)
		aliceQuery := s.InlineQuery(alice, "np")
		bobQuery := s.InlineQuery(bob, "np")

		s.Send(aliceQuery)
		s.Send(bobQuery)

		aliceAnswer := s.Telegram.InlineAnswerTo(t, aliceQuery)
		require.Len(t, aliceAnswer.Results, 1)
		assert.Equal(t, aliceAudio.FileID, aliceAnswer.Results[0].AudioFileID)
		bobAnswer := s.Telegram.InlineAnswerTo(t, bobQuery)
		require.Len(t, bobAnswer.Results, 1)
		assert.Equal(t, bobAudio.FileID, bobAnswer.Results[0].AudioFileID)
	})

	t.Run("link deletes the password message and confirms", func(t *testing.T) {
		s := harness.New(t)
		account := s.Navidrome.CreateAccount(t, "alice")

		link := s.Link(alice, account)

		assert.Equal(t, []string{strconv.Itoa(link.Message.ID)}, s.Telegram.DeletedMessages())
		assert.Contains(t, s.WindowText(), "привязан")
	})

	t.Run("link command is gone", func(t *testing.T) {
		s := harness.New(t)
		account := s.Navidrome.CreateAccount(t, "alice")

		s.Send(s.TextMessage(alice, "/link "+account.Login+" "+account.Password))

		assert.Empty(t, s.Telegram.AllCalls())
	})

	t.Run("screen shows the linked login", func(t *testing.T) {
		s := harness.New(t)
		account := s.LinkNewAccount(alice)

		s.Open(alice, "👤 Аккаунты")

		assert.Contains(t, s.WindowText(), "Привязан аккаунт <b>"+account.Login+"</b>")
		assert.Contains(t, telegram.ButtonTexts(s.Telegram.Buttons(t)), "🔗 Привязать другой")
	})

	t.Run("going back stops waiting for the password", func(t *testing.T) {
		s := harness.New(t)
		account := s.Navidrome.CreateAccount(t, "alice")
		s.Open(alice, "👤 Аккаунты", "🔗 Привязать", "✖️ Отмена")

		s.SendText(alice, account.Login+" "+account.Password)
		s.Open(alice, "👤 Аккаунты")

		assert.Empty(t, s.Telegram.DeletedMessages())
		assert.Contains(t, s.WindowText(), "не привязан")
	})

	t.Run("audio while waiting is uploaded and the wait goes on", func(t *testing.T) {
		s := harness.New(t)
		account := s.Navidrome.CreateAccount(t, "alice")
		s.Open(alice, "👤 Аккаунты", "🔗 Привязать")

		upload := s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.SendText(alice, account.Login+" "+account.Password)

		assert.Equal(t, []string{"👀", "👍"}, s.Telegram.ReactionsOn(t, upload.Message.ID))
		assert.Contains(t, s.WindowText(), "привязан. Теперь")
	})

	t.Run("recent shows the linked account's history", func(t *testing.T) {
		s := harness.New(t)
		account := s.Navidrome.CreateAccount(t, "alice")
		s.Link(alice, account)
		audio := s.UploadAudio("track.mp3")
		s.Send(s.AudioMessage(alice, audio))
		s.Navidrome.Play(t, account, s.Navidrome.IndexedTrack(t, account, s.Library, audiofile.FixtureTitle).ID)
		query := s.InlineQuery(alice, "recent")

		s.Send(query)

		assert.Contains(t, telegram.AudioFileIDs(s.Telegram.InlineAnswerTo(t, query)), audio.FileID)
	})

	t.Run("link with wrong password is rejected", func(t *testing.T) {
		s := harness.New(t)
		account := s.Navidrome.CreateAccount(t, "alice")
		account.Password = "wrong"

		link := s.Link(alice, account)
		query := s.InlineQuery(alice, "np")
		s.Send(query)

		assert.Equal(t, []string{strconv.Itoa(link.Message.ID)}, s.Telegram.DeletedMessages())
		assert.Contains(t, s.WindowText(), "Неверный логин или пароль")
		assertLinkHint(t, s.Telegram.InlineAnswerTo(t, query))
	})

	t.Run("inline without account suggests linking", func(t *testing.T) {
		s := harness.New(t)
		np := s.InlineQuery(alice, "np")
		recent := s.InlineQuery(alice, "recent")

		s.Send(np)
		s.Send(recent)

		assertLinkHint(t, s.Telegram.InlineAnswerTo(t, np))
		assertLinkHint(t, s.Telegram.InlineAnswerTo(t, recent))
	})
}

func assertLinkHint(t *testing.T, answer telegram.InlineAnswer) {
	t.Helper()

	require.Len(t, answer.Results, 1)
	assert.Contains(t, answer.Results[0].Title, "Привяжите аккаунт Navidrome")
}

func TestNavidromeDefaults(t *testing.T) {
	t.Parallel()

	t.Run("account made in Navidrome sees the Shared Library but no Personal one", func(t *testing.T) {
		s := harness.New(t)

		account := s.Navidrome.CreateAccount(t, "dave")

		libraries := s.Navidrome.Libraries(t, account)
		assert.Contains(t, libraries, s.NavidromePath("shared"))
		for _, user := range []harness.User{admin, alice, bob} {
			assert.NotContains(t, libraries, s.NavidromePath(s.PersonalDir(user)))
		}
	})
}

func TestLinkAfterMove(t *testing.T) {
	t.Parallel()

	t.Run("account linked to another user is refused", func(t *testing.T) {
		s := harness.New(t)
		account := s.LinkNewAccount(alice)

		s.Link(bob, account)

		assert.Contains(t, s.WindowText(), "Этот аккаунт Navidrome уже привязан к другому пользователю")
		assert.Equal(t, []string{
			s.NavidromePath("shared"),
			s.NavidromePath(s.PersonalDir(alice)),
		}, s.Navidrome.Libraries(t, account))
	})

	t.Run("login case does not get around another user's link", func(t *testing.T) {
		s := harness.New(t)
		account := s.LinkNewAccount(alice)
		shouted := navidrome.Account{Login: strings.ToUpper(account.Login), Password: account.Password}

		s.Link(bob, shouted)

		assert.Contains(t, s.WindowText(), "уже привязан к другому пользователю")
		assert.Contains(t, s.Navidrome.Libraries(t, account), s.NavidromePath(s.PersonalDir(alice)))
	})

	t.Run("relinking one's own account works", func(t *testing.T) {
		s := harness.New(t)
		account := s.LinkNewAccount(alice)

		s.Link(alice, account)

		assert.Contains(t, s.WindowText(), "привязан. Теперь")
	})

	t.Run("link warns and hides others' Personal Libraries", func(t *testing.T) {
		s := harness.New(t)
		s.Send(s.AudioMessage(bob, s.UploadAudio("track.mp3")))
		s.WaitIngest()
		account := s.Navidrome.CreateAccount(t, "alice")
		s.Navidrome.GrantAllLibraries(t, account)
		bobAccount := s.LinkNewAccount(bob)
		s.Navidrome.IndexedTrack(t, bobAccount, s.Library, audiofile.FixtureTitle)

		s.Link(alice, account)

		assert.Contains(t, s.WindowText(), "только вашу личную библиотеку и общую")
		assert.Empty(t, s.Navidrome.SearchFor(t, account, s.Library, audiofile.FixtureTitle))
	})
}

func TestRegistration(t *testing.T) {
	t.Parallel()

	t.Run("invited person chooses a Navidrome login", func(t *testing.T) {
		s := harness.New(t)
		carol := harness.Newcomer("carol")
		s.Send(s.TextMessage(carol, "/start "+s.Invite()))
		assert.Contains(t, s.WindowText(), "Придумайте логин")

		s.SendText(carol, carol.Username)

		account := s.IssuedAccount()
		assert.Equal(t, carol.Username, account.Login)
		assert.True(t, s.Navidrome.CanLogin(account))
		assert.Contains(t, s.WindowText(), "Привет")
	})

	t.Run("password comes in a message of its own", func(t *testing.T) {
		s := harness.New(t)
		carol := harness.Newcomer("carol")
		s.Send(s.TextMessage(carol, "/start "+s.Invite()))
		window := s.Telegram.Window().MessageID

		s.SendText(carol, carol.Username)

		assert.Equal(t, window, s.Telegram.Window().MessageID)
		assert.NotContains(t, s.WindowText(), "tg-spoiler")
		assert.Contains(t, s.LastReply().Text, "tg-spoiler")
	})

	t.Run("taken login makes the bot ask for another", func(t *testing.T) {
		s := harness.New(t)
		taken := s.Navidrome.CreateAccount(t, "carol")
		carol := harness.Newcomer("carol")
		s.Send(s.TextMessage(carol, "/start "+s.Invite()))
		s.SendText(carol, taken.Login)
		assert.Contains(t, s.WindowText(), "уже занят")
		login := navidrome.UniqueLogin("carol")

		s.SendText(carol, login)

		account := s.IssuedAccount()
		assert.Equal(t, login, account.Login)
		assert.True(t, s.Navidrome.CanLogin(account))
		assert.True(t, s.Navidrome.CanLogin(taken))
	})

	t.Run("unfit login makes the bot ask for another", func(t *testing.T) {
		s := harness.New(t)
		carol := harness.Newcomer("carol")
		s.Send(s.TextMessage(carol, "/start "+s.Invite()))

		s.SendText(carol, "no")

		assert.Contains(t, s.WindowText(), "Такой логин не подойдёт")
	})

	t.Run("login answer after restart is processed", func(t *testing.T) {
		s := harness.New(t)
		carol := harness.Newcomer("carol")
		s.Send(s.TextMessage(carol, "/start "+s.Invite()))

		s.Restart()
		s.SendText(carol, carol.Username)

		account := s.IssuedAccount()
		assert.Equal(t, carol.Username, account.Login)
		assert.True(t, s.Navidrome.CanLogin(account))
	})

	t.Run("existing account links and leads home", func(t *testing.T) {
		s := harness.New(t)
		carol := harness.Newcomer("carol")
		account := s.Navidrome.CreateAccount(t, "carol")
		s.Send(s.TextMessage(carol, "/start "+s.Invite()))
		s.Go(carol, "🔗 У меня уже есть аккаунт")

		s.SendText(carol, account.Login+" "+account.Password)
		replies := len(s.Telegram.Replies(t))
		s.SendText(carol, navidrome.UniqueLogin("carol"))

		assert.Contains(t, s.WindowText(), "Привет")
		assert.Contains(t, s.WindowText(), "привязан")
		assert.Len(t, s.Telegram.Replies(t), replies)
	})

	t.Run("registered user sees their np", func(t *testing.T) {
		s := harness.New(t)
		carol := harness.Newcomer("carol")
		account := s.Register(carol)
		audio := s.UploadAudio("track.mp3")
		s.Send(s.AudioMessage(carol, audio))
		s.Navidrome.StartPlaying(t, account, s.Navidrome.IndexedTrack(t, account, s.Library, audiofile.FixtureTitle).ID)
		query := s.InlineQuery(carol, "np")

		s.Send(query)

		answer := s.Telegram.InlineAnswerTo(t, query)
		require.Len(t, answer.Results, 1)
		assert.Equal(t, audio.FileID, answer.Results[0].AudioFileID)
	})

	t.Run("newcomer's account sees only their own and the Shared Library", func(t *testing.T) {
		s := harness.New(t)
		carol := harness.Newcomer("carol")

		account := s.Register(carol)

		assert.Equal(t, []string{
			s.NavidromePath("shared"),
			s.NavidromePath(s.PersonalDir(carol)),
		}, s.Navidrome.Libraries(t, account))
	})
}
