package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/audiofile"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/telegram"
)

func TestTake(t *testing.T) {
	t.Parallel()

	t.Run("taken track lands in the taker's Personal Library", func(t *testing.T) {
		s := harness.New(t)
		bobAccount := s.LinkNewAccount(bob)
		s.Share(alice, s.Uploaded(alice, s.UploadAudio("track.mp3")), "🔗 Трек")

		s.Take(bob, 1)

		assert.Equal(t, []string{audiofile.FixtureTrackPath}, s.PersonalFiles(bob))
		s.Navidrome.IndexedTrack(t, bobAccount, s.PersonalPath(bob, ""), audiofile.FixtureTitle)
		assert.Contains(t, s.LastCallbackAnswer(), "в вашей библиотеке")
	})

	t.Run("taken track stays after the author unshares", func(t *testing.T) {
		s := harness.New(t)
		s.Share(alice, s.Uploaded(alice, s.UploadAudio("track.mp3")), "🔗 Трек")
		aliceButtons := s.Telegram.Buttons(t)
		s.Take(bob, 1)

		s.Press(alice, telegram.ButtonNamed(t, aliceButtons, "🔒 Снять Share"))

		assert.Empty(t, s.SharedFiles())
		assert.Equal(t, []string{audiofile.FixtureTrackPath}, s.PersonalFiles(bob))
	})

	t.Run("taker reshares after the author unshares", func(t *testing.T) {
		s := harness.New(t)
		audio := s.UploadAudio("track.mp3")
		s.Share(alice, s.Uploaded(alice, audio), "🔗 Трек")
		aliceButtons := s.Telegram.Buttons(t)
		s.Take(bob, 1)
		s.Press(alice, telegram.ButtonNamed(t, aliceButtons, "🔒 Снять Share"))

		s.Share(bob, s.BotAudio(bob, audio), "🔗 Трек")

		assert.Equal(t, []string{audiofile.FixtureTrackPath}, s.SharedFiles())
		assert.Contains(t, feed(s, alice), "@bob")
		assert.NotContains(t, feed(s, alice), "@alice")
	})

	t.Run("upload of a shared file needs no download", func(t *testing.T) {
		s := harness.New(t)
		audio := s.UploadAudio("track.mp3")
		s.Share(alice, s.Uploaded(alice, audio), "🔗 Трек")
		s.Telegram.Forget()
		forward := s.AudioMessage(bob, audio)

		s.Send(forward)
		s.WaitIngest()

		assert.Empty(t, s.Telegram.CallsTo("getFile"))
		assert.Equal(t, []string{audiofile.FixtureTrackPath}, s.PersonalFiles(bob))
		assert.Equal(t, []string{"👀", "👍"}, s.Telegram.ReactionsOn(t, forward.Message.ID))
		assert.Empty(t, s.Telegram.Replies(t))
	})

	t.Run("shared file of a track the user owns is not copied", func(t *testing.T) {
		s := harness.New(t)
		mp3 := audiofile.Generate(t, "song.mp3", audiofile.Spec{Tags: audiofile.SongTags})
		s.Uploaded(bob, s.UploadAudioFile(mp3))
		aliceAudio := s.UploadAudioFile(mp3)
		s.Share(alice, s.Uploaded(alice, aliceAudio), "🔗 Трек")
		s.Telegram.Forget()
		forward := s.AudioMessage(bob, aliceAudio)

		s.Send(forward)
		s.WaitIngest()

		assert.Equal(t, []string{"Artist/Album/01 - Dup Song.mp3"}, s.PersonalFiles(bob))
		harness.AssertAlreadyExists(t, s, forward.Message.ID)
	})

	t.Run("user's own version of a shared track is stored", func(t *testing.T) {
		s := harness.New(t)
		mp3 := audiofile.Generate(t, "song.mp3", audiofile.Spec{Tags: audiofile.SongTags})
		s.Share(alice, s.Uploaded(alice, s.UploadAudioFile(mp3)), "🔗 Трек")
		flac := s.UploadDocument(audiofile.Generate(t, "song.flac", audiofile.Spec{Tags: audiofile.SongTags}), "audio/flac")
		s.Telegram.Forget()

		s.Send(s.DocumentMessage(bob, flac))
		s.WaitIngest()

		assert.Equal(t, []string{"Artist/Album/01 - Dup Song.flac"}, s.PersonalFiles(bob))
		assert.Equal(t, []string{"Artist/Album/01 - Dup Song.mp3"}, s.SharedFiles())
		assert.Empty(t, s.Telegram.Replies(t))
	})

	t.Run("upload of a track private to another user reveals nothing", func(t *testing.T) {
		s := harness.New(t)
		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.Telegram.Forget()

		s.Uploaded(bob, s.UploadAudio("track.mp3"))

		assert.NotEmpty(t, s.Telegram.CallsTo("getFile"))
		assert.Equal(t, []string{audiofile.FixtureTrackPath}, s.PersonalFiles(bob))
		assert.Empty(t, s.Telegram.Replies(t))
	})

	t.Run("send file button sends the audio", func(t *testing.T) {
		s := harness.New(t)
		audio := s.UploadAudio("track.mp3")
		s.Share(alice, s.Uploaded(alice, audio), "🔗 Трек")
		s.Send(s.TextMessage(bob, "/shared"))

		s.Press(bob, s.Button("1. ▶️ Прислать файл"))

		sent := s.Telegram.CallsTo("sendAudio")
		require.Len(t, sent, 1)
		assert.Equal(t, audio.FileID, sent[0].Params["audio"])
		assert.Equal(t, "1002", sent[0].Params["chat_id"])
	})
}

func TestSharedFeed(t *testing.T) {
	t.Parallel()

	t.Run("inline shared lists recent Shares with their authors", func(t *testing.T) {
		s := harness.New(t)
		aliceAudio := s.UploadAudio("track.mp3")
		s.Share(alice, s.Uploaded(alice, aliceAudio), "🔗 Трек")
		bobAudio := s.UploadAudioFile(audiofile.Generate(t, "song.mp3", audiofile.Spec{Tags: audiofile.SongTags}))
		s.Share(bob, s.Uploaded(bob, bobAudio), "🔗 Трек")
		query := s.InlineQuery(alice, "shared")

		s.Send(query)

		results := s.Telegram.InlineAnswerTo(t, query).Results
		require.Len(t, results, 3)
		assert.Equal(t, bobAudio.FileID, results[1].AudioFileID)
		assert.Contains(t, results[1].Caption, "@bob")
		assert.Equal(t, aliceAudio.FileID, results[2].AudioFileID)
		assert.Contains(t, results[2].Caption, "@alice")
	})

	t.Run("empty feed says nobody has shared yet", func(t *testing.T) {
		s := harness.New(t)

		s.Send(s.TextMessage(alice, "/shared"))

		assert.Contains(t, s.LastReply().Text, "Пока никто ничего не расшарил")
	})
}

func feed(s *harness.Scenario, user harness.User) string {
	s.Send(s.TextMessage(user, "/shared"))
	return s.LastReply().Text
}

func TestAuthorName(t *testing.T) {
	t.Parallel()

	t.Run("admin from config is shown by username", func(t *testing.T) {
		s := harness.New(t)
		s.Share(admin, s.Uploaded(admin, s.UploadAudio("track.mp3")), "🔗 Трек")

		assert.Contains(t, feed(s, alice), "@"+admin.Username)
	})

	t.Run("renamed user is shown by the new username", func(t *testing.T) {
		s := harness.New(t)
		s.Share(alice, s.Uploaded(alice, s.UploadAudio("track.mp3")), "🔗 Трек")
		renamed := harness.User{ID: alice.ID, Username: "alice_new"}

		s.Send(s.TextMessage(renamed, "/start"))

		assert.Contains(t, feed(s, bob), "@alice_new")
	})
}
