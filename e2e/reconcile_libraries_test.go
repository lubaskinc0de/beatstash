package e2e

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/audiofile"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/navidrome"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/telegram"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

// quota makes the home screen tell the Usage.
const quota = library.Quota(1 << 30)

func TestReconciliation(t *testing.T) {
	t.Parallel()

	t.Run("file removed by hand takes its Track and its weight away", func(t *testing.T) {
		s := harness.New(t, harness.WithDefaultQuota(quota))
		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.RemoveByHand(s.PersonalPath(alice, audiofile.FixtureTrackPath))

		s.Restart()

		s.Open(alice)
		assert.Contains(t, s.WindowText(), s.Catalog(alice).Home(alice.Username, telegram.BotUsername, library.Usage{Quota: quota}))
		s.Telegram.Forget()
		again := s.AudioMessage(alice, s.UploadAudio("track.mp3"))
		s.Send(again)
		s.WaitIngest()
		assert.Equal(t, []string{"👀", "👍"}, s.Telegram.ReactionsOn(t, again.Message.ID))
		assert.Empty(t, s.Telegram.Replies(t))
		assert.Equal(t, []string{audiofile.FixtureTrackPath}, s.PersonalFiles(alice))
	})

	t.Run("bot notices a removed file every reconcile_interval", func(t *testing.T) {
		s := harness.New(t, harness.WithDefaultQuota(quota), harness.WithReconcileInterval(100*time.Millisecond))
		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.RemoveByHand(s.PersonalPath(alice, audiofile.FixtureTrackPath))

		s.WaitReconcile()

		s.Open(alice)
		assert.Contains(t, s.WindowText(), s.Catalog(alice).Home(alice.Username, telegram.BotUsername, library.Usage{Quota: quota}))
	})

	t.Run("tags written by hand take an Inbox Track out of the Inbox", func(t *testing.T) {
		s := harness.New(t)
		upload := s.Uploaded(alice, s.UploadAudioFile(audiofile.Generate(t, "audio_1.mp3", audiofile.Spec{})))
		s.RetagByHand(s.PersonalPath(alice, "Inbox/audio_1.mp3"), map[string]string{"ARTIST": "Hand Artist", "TITLE": "Hand Title"})

		s.Restart()
		s.Share(alice, upload, s.Catalog(alice).ShareTrack())

		assert.Equal(t, []string{"Inbox/audio_1.mp3"}, s.SharedFiles())
		s.Open(bob, s.Catalog(bob).MusicButton())
		assert.Contains(t, telegram.ButtonTexts(s.Telegram.Buttons(t)), "Hand Artist — Hand Title")
	})

	t.Run("file replaced by hand brings its Quality and size", func(t *testing.T) {
		s := harness.New(t, harness.WithDefaultQuota(quota))
		s.Uploaded(alice, s.UploadAudioFile(audiofile.Generate(t, "low.mp3", audiofile.Spec{Bitrate: "128k", Tags: audiofile.SongTags})))
		high := audiofile.Generate(t, "high.mp3", audiofile.Spec{Bitrate: "320k", Tags: audiofile.SongTags})
		path := s.PersonalPath(alice, "Artist/Album/01 - Dup Song.mp3")
		s.WriteByHand(path, high)

		s.Restart()

		s.Open(alice)
		used := library.Usage{Used: harness.FileSize(t, path), Quota: quota}
		assert.Contains(t, s.WindowText(), s.Catalog(alice).Home(alice.Username, telegram.BotUsername, used))
		s.Telegram.Forget()
		again := s.AudioMessage(alice, s.UploadAudioFile(high))
		s.Send(again)
		s.WaitIngest()
		harness.AssertAlreadyExists(t, s, alice, again.Message.ID)
	})

	t.Run("feed sends the file replaced by hand", func(t *testing.T) {
		s := harness.New(t)
		upload := s.Uploaded(alice, s.UploadAudioFile(audiofile.Generate(t, "low.mp3", audiofile.Spec{Bitrate: "128k", Tags: audiofile.SongTags})))
		s.Share(alice, upload, s.Catalog(alice).ShareTrack())
		high := audiofile.Generate(t, "high.mp3", audiofile.Spec{Bitrate: "320k", Tags: audiofile.SongTags})
		s.WriteByHand(s.PersonalPath(alice, "Artist/Album/01 - Dup Song.mp3"), high)

		s.Restart()
		s.OpenShared(bob, 1)
		s.Press(bob, s.Button(s.Catalog(bob).SendFileButton()))

		sent := s.Telegram.CallsTo("sendAudio")
		require.Len(t, sent, 1)
		assert.Equal(t, "Dup Song.mp3", sent[0].Params[telegram.UploadedFileParam])
	})

	t.Run("unreadable audio file does not stop the rest", func(t *testing.T) {
		s := harness.New(t, harness.WithDefaultQuota(quota))
		first := audiofile.Generate(t, "first.mp3", audiofile.Spec{Tags: map[string]string{"artist": "A", "title": "First"}})
		second := audiofile.Generate(t, "second.mp3", audiofile.Spec{Tags: map[string]string{"artist": "A", "title": "Second"}})
		s.Uploaded(alice, s.UploadAudioFile(first))
		s.Uploaded(alice, s.UploadAudioFile(second))
		firstPath := s.PersonalPath(alice, "A/Singles/First.mp3")
		weight := harness.FileSize(t, firstPath)
		s.WriteByHand(firstPath, audiofile.WriteFile(t, "garbage.mp3", []byte(strings.Repeat("not an mp3 ", 100))))
		s.RemoveByHand(s.PersonalPath(alice, "A/Singles/Second.mp3"))

		s.Restart()

		s.Open(alice)
		assert.Contains(t, s.WindowText(), s.Catalog(alice).Home(alice.Username, telegram.BotUsername, library.Usage{Used: weight, Quota: quota}))
	})

	t.Run("Attached Library keeps its Tracks", func(t *testing.T) {
		var account navidrome.Account
		s := harness.NewOwnNavidrome(t, func(s *harness.Scenario) {
			account = s.Navidrome.CreateAccount(t, "alice")
			own := s.NewNavidromeLibrary("own", audiofile.Fixture("track.mp3"))
			s.Navidrome.OpenLibrary(t, account, own.ID)
			s.Navidrome.UntilSongs(t, own.ID, 1)
		})

		s.Restart()
		s.Link(alice, account)

		assert.Contains(t, s.WindowText(), s.Catalog(alice).Linked(account.Login, 1))
	})

	t.Run("file renamed by hand stays the same Track", func(t *testing.T) {
		s := harness.New(t)
		upload := s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.MoveByHand(s.PersonalPath(alice, audiofile.FixtureTrackPath), s.PersonalPath(alice, "Mine/renamed.mp3"))

		s.Restart()
		s.Share(alice, upload, s.Catalog(alice).ShareTrack())

		assert.Equal(t, []string{"Mine/renamed.mp3"}, s.SharedFiles())
		s.Telegram.Forget()
		again := s.AudioMessage(alice, s.UploadAudio("track.mp3"))
		s.Send(again)
		s.WaitIngest()
		harness.AssertAlreadyExists(t, s, alice, again.Message.ID)
		assert.Equal(t, []string{"Mine/renamed.mp3"}, s.PersonalFiles(alice))
	})

	t.Run("shared file renamed in the Personal Library keeps its Share", func(t *testing.T) {
		s := harness.New(t)
		upload := s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.Share(alice, upload, s.Catalog(alice).ShareTrack())
		s.MoveByHand(s.PersonalPath(alice, audiofile.FixtureTrackPath), s.PersonalPath(alice, "Mine/renamed.mp3"))

		s.Restart()
		s.Send(s.ReplyCommand(alice, "/share", upload))

		assert.Contains(t, telegram.ButtonTexts(s.Telegram.Buttons(t)), s.Catalog(alice).UnshareTrack())
		assert.Equal(t, []string{audiofile.FixtureTrackPath}, s.SharedFiles())
	})

	t.Run("file put by hand becomes a Track of the Library", func(t *testing.T) {
		s := harness.New(t, harness.WithDefaultQuota(quota))
		path := s.PersonalPath(alice, "Dropped/song.mp3")
		s.WriteByHand(path, audiofile.Fixture("track.mp3"))

		s.Restart()

		s.Open(alice)
		used := library.Usage{Used: harness.FileSize(t, path), Quota: quota}
		assert.Contains(t, s.WindowText(), s.Catalog(alice).Home(alice.Username, telegram.BotUsername, used))
		s.Telegram.Forget()
		again := s.AudioMessage(alice, s.UploadAudio("track.mp3"))
		s.Send(again)
		s.WaitIngest()
		harness.AssertAlreadyExists(t, s, alice, again.Message.ID)
		assert.Equal(t, []string{"Dropped/song.mp3"}, s.PersonalFiles(alice))
	})

	t.Run("file put by hand over the Quota stays and the next Ingest is refused", func(t *testing.T) {
		size := harness.FileSize(t, audiofile.Fixture("track.mp3"))
		small := library.Quota(size / 2)
		s := harness.New(t, harness.WithDefaultQuota(small))
		s.WriteByHand(s.PersonalPath(alice, "Dropped/song.mp3"), audiofile.Fixture("track.mp3"))

		s.Restart()
		upload := s.AudioMessage(alice, s.UploadAudioFile(audiofile.Generate(t, "other.mp3", audiofile.Spec{Tags: audiofile.SongTags})))
		s.Send(upload)
		s.WaitIngest()

		assert.Equal(t, []string{"Dropped/song.mp3"}, s.PersonalFiles(alice))
		assert.Equal(t, s.Catalog(alice).NoRoom(library.Usage{Used: size, Quota: small}, ""), s.LastReply().Text)
	})

	t.Run("file put by hand into the Shared Library does not become a Track", func(t *testing.T) {
		s := harness.New(t)
		s.WriteByHand(s.SharedPath(audiofile.FixtureTrackPath), audiofile.Fixture("track.mp3"))

		s.Restart()
		s.Share(alice, s.Uploaded(alice, s.UploadAudio("track.mp3")), s.Catalog(alice).ShareTrack())

		assert.Equal(t, []string{
			"Fixture Artist/Fixture Album/01 - Fixture Song (2).mp3",
			audiofile.FixtureTrackPath,
		}, s.SharedFiles())
	})

	t.Run("file removed by hand from the Shared Library leaves the feed and keeps the Takes", func(t *testing.T) {
		s := harness.New(t)
		s.Share(alice, s.Uploaded(alice, s.UploadAudio("track.mp3")), s.Catalog(alice).ShareTrack())
		s.Take(bob, 1)
		s.RemoveByHand(s.SharedPath(audiofile.FixtureTrackPath))

		s.Restart()

		s.Open(bob, s.Catalog(bob).MusicButton())
		assert.Contains(t, s.WindowText(), s.Catalog(bob).FeedEmpty())
		assert.Equal(t, []string{audiofile.FixtureTrackPath}, s.PersonalFiles(bob))
		assert.Equal(t, []string{"1. @alice — 1"}, topLines(t, top(s, alice), s.Catalog(alice).TopTakenLabel(), s.Catalog(alice).TopAllTimeLabel()))
	})

	t.Run("Take counts for the author after the taker removes its file by hand", func(t *testing.T) {
		s := harness.New(t)
		s.Share(alice, s.Uploaded(alice, s.UploadAudio("track.mp3")), s.Catalog(alice).ShareTrack())
		s.Take(bob, 1)
		s.RemoveByHand(s.PersonalPath(bob, audiofile.FixtureTrackPath))

		s.Restart()

		assert.Equal(t, []string{"1. @alice — 1"}, topLines(t, top(s, alice), s.Catalog(alice).TopTakenLabel(), s.Catalog(alice).TopAllTimeLabel()))
	})

	t.Run("file removed by hand from the author's Personal Library keeps the Share", func(t *testing.T) {
		s := harness.New(t)
		s.Share(alice, s.Uploaded(alice, s.UploadAudio("track.mp3")), s.Catalog(alice).ShareTrack())
		s.RemoveByHand(s.PersonalPath(alice, audiofile.FixtureTrackPath))

		s.Restart()

		assert.Equal(t, []string{audiofile.FixtureTrackPath}, s.SharedFiles())
		s.Open(bob, s.Catalog(bob).MusicButton())
		assert.Contains(t, telegram.ButtonTexts(s.Telegram.Buttons(t)), fixtureButton)
	})
}
