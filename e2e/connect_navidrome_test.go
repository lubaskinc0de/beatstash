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
		assert.Contains(t, s.WindowText(), s.Catalog(alice).Linked(account.Login, 0))
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

		s.Open(alice, s.Catalog(alice).AccountsButton())

		assert.Contains(t, s.WindowText(), s.Catalog(alice).NavidromeLinked(account.Login))
		assert.Contains(t, telegram.ButtonTexts(s.Telegram.Buttons(t)), s.Catalog(alice).LinkAnother())
	})

	t.Run("going back stops waiting for the password", func(t *testing.T) {
		s := harness.New(t)
		account := s.Navidrome.CreateAccount(t, "alice")
		s.Open(alice, s.Catalog(alice).AccountsButton(), s.Catalog(alice).Link(), s.Catalog(alice).Cancel())

		s.SendText(alice, account.Login+" "+account.Password)
		s.Open(alice, s.Catalog(alice).AccountsButton())

		assert.Empty(t, s.Telegram.DeletedMessages())
		assert.Contains(t, s.WindowText(), s.Catalog(alice).NavidromeNotLinked())
	})

	t.Run("audio while waiting is uploaded and the wait goes on", func(t *testing.T) {
		s := harness.New(t)
		account := s.Navidrome.CreateAccount(t, "alice")
		s.Open(alice, s.Catalog(alice).AccountsButton(), s.Catalog(alice).Link())

		upload := s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.SendText(alice, account.Login+" "+account.Password)

		assert.Equal(t, []string{"👀", "👍"}, s.Telegram.ReactionsOn(t, upload.Message.ID))
		assert.Contains(t, s.WindowText(), s.Catalog(alice).Linked(account.Login, 0))
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
		assert.Contains(t, s.WindowText(), s.Catalog(alice).LinkWrongPassword())
		assertLinkHint(t, s, alice, s.Telegram.InlineAnswerTo(t, query))
	})

	t.Run("link with missing password is rejected and the message is deleted", func(t *testing.T) {
		s := harness.New(t)
		s.Open(alice, s.Catalog(alice).AccountsButton(), s.Catalog(alice).Link())

		input := s.SendText(alice, "alice")

		assert.Contains(t, s.WindowText(), s.Catalog(alice).LinkMalformed())
		assert.Equal(t, []string{strconv.Itoa(input.Message.ID)}, s.Telegram.DeletedMessages())
	})

	t.Run("recent returns a cached document for a played Telegram document", func(t *testing.T) {
		s := harness.New(t)
		account := s.LinkNewAccount(alice)
		document := s.UploadDocument(
			audiofile.Generate(t, "song.flac", audiofile.Spec{Tags: audiofile.SongTags}),
			"audio/flac",
		)
		s.Send(s.DocumentMessage(alice, document))
		s.WaitIngest()
		track := s.Navidrome.IndexedTrack(t, account, s.PersonalPath(alice, ""), "Dup Song")
		s.Navidrome.Play(t, account, track.ID)
		query := s.InlineQuery(alice, "recent")

		s.Send(query)

		results := s.Telegram.InlineAnswerTo(t, query).Results
		require.Len(t, results, 2)
		assert.Equal(t, "document", results[1].Type)
		assert.Equal(t, document.FileID, results[1].DocumentFileID)
	})

	t.Run("inline without account suggests linking", func(t *testing.T) {
		s := harness.New(t)
		np := s.InlineQuery(alice, "np")
		recent := s.InlineQuery(alice, "recent")

		s.Send(np)
		s.Send(recent)

		assertLinkHint(t, s, alice, s.Telegram.InlineAnswerTo(t, np))
		assertLinkHint(t, s, alice, s.Telegram.InlineAnswerTo(t, recent))
	})
}

func assertLinkHint(t *testing.T, s *harness.Scenario, user harness.User, answer telegram.InlineAnswer) {
	t.Helper()

	require.Len(t, answer.Results, 1)
	assert.Contains(t, answer.Results[0].Title, s.Catalog(user).NoNavidromeAccount(telegram.BotUsername).Title)
}

func TestNavidromeDefaults(t *testing.T) {
	t.Parallel()

	t.Run("account made in Navidrome sees the Shared Library but no Personal one", func(t *testing.T) {
		s := harness.New(t)

		account := s.Navidrome.CreateAccountWithDefaults(t, "dave")

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

		assert.Contains(t, s.WindowText(), s.Catalog(bob).LinkTaken())
		assert.Equal(t, []string{
			navidrome.RootLibraryPath,
			s.NavidromePath("shared"),
			s.NavidromePath(s.PersonalDir(alice)),
		}, s.Navidrome.Libraries(t, account))
	})

	t.Run("login case does not get around another user's link", func(t *testing.T) {
		s := harness.New(t)
		account := s.LinkNewAccount(alice)
		shouted := navidrome.Account{Login: strings.ToUpper(account.Login), Password: account.Password}

		s.Link(bob, shouted)

		assert.Contains(t, s.WindowText(), s.Catalog(bob).LinkTaken())
		assert.Contains(t, s.Navidrome.Libraries(t, account), s.NavidromePath(s.PersonalDir(alice)))
	})

	t.Run("relinking one's own account works", func(t *testing.T) {
		s := harness.New(t)
		account := s.LinkNewAccount(alice)

		s.Link(alice, account)

		assert.Contains(t, s.WindowText(), s.Catalog(alice).Linked(account.Login, 0))
	})

	t.Run("link keeps the account's own library", func(t *testing.T) {
		s := harness.New(t)
		account := s.Navidrome.CreateAccount(t, "alice")
		own := s.NewNavidromeLibrary("own")
		s.Navidrome.OpenLibrary(t, account, own.ID)

		s.Link(alice, account)

		assert.Equal(t, []string{
			navidrome.RootLibraryPath,
			own.Path,
			s.NavidromePath("shared"),
			s.NavidromePath(s.PersonalDir(alice)),
		}, s.Navidrome.Libraries(t, account))
	})

	t.Run("link warns and hides others' Personal Libraries", func(t *testing.T) {
		s := harness.New(t)
		s.Send(s.AudioMessage(bob, s.UploadAudio("track.mp3")))
		s.WaitIngest()
		account := s.Navidrome.CreateAccount(t, "alice")
		s.Navidrome.OpenLibrary(t, account, s.Navidrome.LibraryAt(t, s.NavidromePath(s.PersonalDir(bob))))
		bobAccount := s.LinkNewAccount(bob)
		s.Navidrome.IndexedTrack(t, bobAccount, s.Library, audiofile.FixtureTitle)

		s.Link(alice, account)

		assert.Contains(t, s.WindowText(), s.Catalog(alice).Linked(account.Login, 0))
		assert.Empty(t, s.Navidrome.SearchFor(t, account, s.Library, audiofile.FixtureTitle))
	})
}

func TestRegistration(t *testing.T) {
	t.Parallel()

	t.Run("invited person chooses a Navidrome login", func(t *testing.T) {
		s := harness.New(t)
		carol := harness.Newcomer("carol")
		s.Send(s.TextMessage(carol, "/start "+s.Invite()))
		assert.Contains(t, s.WindowText(), s.Catalog(carol).ChooseLogin())

		s.SendText(carol, carol.Username)

		account := s.IssuedAccount()
		assert.Equal(t, carol.Username, account.Login)
		assert.True(t, s.Navidrome.CanLogin(account))
		assert.Contains(t, s.WindowText(), s.Catalog(carol).Home(carol.Username, telegram.BotUsername))
	})

	t.Run("password comes in a message of its own", func(t *testing.T) {
		s := harness.New(t)
		carol := harness.Newcomer("carol")
		s.Send(s.TextMessage(carol, "/start "+s.Invite()))

		s.SendText(carol, carol.Username)

		assert.NotContains(t, s.WindowText(), "tg-spoiler")
		assert.Contains(t, s.LastReply().Text, "tg-spoiler")
	})

	t.Run("taken login makes the bot ask for another", func(t *testing.T) {
		s := harness.New(t)
		taken := s.Navidrome.CreateAccount(t, "carol")
		carol := harness.Newcomer("carol")
		s.Send(s.TextMessage(carol, "/start "+s.Invite()))
		s.SendText(carol, taken.Login)
		assert.Contains(t, s.WindowText(), s.Catalog(carol).LoginTaken(taken.Login))
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

		assert.Contains(t, s.WindowText(), s.Catalog(carol).LoginInvalid())
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
		s.Go(carol, s.Catalog(carol).HaveAccount())

		s.SendText(carol, account.Login+" "+account.Password)
		replies := len(s.Telegram.Replies(t))
		s.SendText(carol, navidrome.UniqueLogin("carol"))

		assert.Contains(t, s.WindowText(), s.Catalog(carol).Home(carol.Username, telegram.BotUsername))
		assert.Contains(t, s.WindowText(), s.Catalog(carol).Linked(account.Login, 0))
		assert.Len(t, s.Telegram.Replies(t), replies)
	})

	t.Run("registered user sees their np", func(t *testing.T) {
		s := harness.NewOwnNavidrome(t, nil)
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

	t.Run("newcomer's account sees their own and the Shared Library", func(t *testing.T) {
		s := harness.NewOwnNavidrome(t, nil)
		carol := harness.Newcomer("carol")

		account := s.Register(carol)

		assert.Equal(t, []string{
			navidrome.RootLibraryPath,
			s.NavidromePath("shared"),
			s.NavidromePath(s.PersonalDir(carol)),
		}, s.Navidrome.Libraries(t, account))
	})
}
