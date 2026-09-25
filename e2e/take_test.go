package e2e

import (
	"strconv"
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTake(t *testing.T) {
	t.Parallel()

	t.Run("taken track lands in the taker's Personal Library", func(t *testing.T) {
		s := newScenario(t)
		bobAccount := s.linkNewAccount(bob)
		s.share(alice, s.uploaded(alice, s.uploadAudio("track.mp3")), "🔗 Трек")

		s.take(bob, 1)

		assert.Equal(t, []string{fixtureTrackPath}, s.personalFiles(bob))
		env.navidrome.indexedTrack(t, bobAccount, s.personalPath(bob, ""), fixtureTitle)
		assert.Contains(t, lastCallbackAnswer(t, s), "в вашей библиотеке")
	})

	t.Run("taken track stays after the author unshares", func(t *testing.T) {
		s := newScenario(t)
		s.share(alice, s.uploaded(alice, s.uploadAudio("track.mp3")), "🔗 Трек")
		aliceButtons := s.botAPI.buttons(t)
		s.take(bob, 1)

		s.press(alice, buttonIn(t, aliceButtons, "🔒 Снять Share"))

		assert.Empty(t, s.sharedFiles())
		assert.Equal(t, []string{fixtureTrackPath}, s.personalFiles(bob))
	})

	t.Run("taker reshares after the author unshares", func(t *testing.T) {
		s := newScenario(t)
		audio := s.uploadAudio("track.mp3")
		s.share(alice, s.uploaded(alice, audio), "🔗 Трек")
		aliceButtons := s.botAPI.buttons(t)
		s.take(bob, 1)
		s.press(alice, buttonIn(t, aliceButtons, "🔒 Снять Share"))

		s.share(bob, s.botAudio(bob, audio), "🔗 Трек")

		assert.Equal(t, []string{fixtureTrackPath}, s.sharedFiles())
		assert.Contains(t, s.feed(alice), "@bob")
		assert.NotContains(t, s.feed(alice), "@alice")
	})

	t.Run("upload of a shared file needs no download", func(t *testing.T) {
		s := newScenario(t)
		audio := s.uploadAudio("track.mp3")
		s.share(alice, s.uploaded(alice, audio), "🔗 Трек")
		s.botAPI.forget()
		forward := s.audioMessage(bob, audio)

		s.send(forward)
		s.waitIngest()

		assert.Empty(t, s.botAPI.callsTo("getFile"))
		assert.Equal(t, []string{fixtureTrackPath}, s.personalFiles(bob))
		assert.Equal(t, []string{"👀", "👍"}, s.botAPI.reactionsOn(t, forward.Message.ID))
		assert.Empty(t, s.botAPI.replies(t))
	})

	t.Run("shared file of a track the user owns is not copied", func(t *testing.T) {
		s := newScenario(t)
		mp3 := makeAudio(t, "song.mp3", audioSpec{Tags: songTags})
		s.uploaded(bob, s.uploadAudioFile(mp3))
		aliceAudio := s.uploadAudioFile(mp3)
		s.share(alice, s.uploaded(alice, aliceAudio), "🔗 Трек")
		s.botAPI.forget()
		forward := s.audioMessage(bob, aliceAudio)

		s.send(forward)
		s.waitIngest()

		assert.Equal(t, []string{"Artist/Album/01 - Dup Song.mp3"}, s.personalFiles(bob))
		assertAlreadyExists(t, s, forward.Message.ID)
	})

	t.Run("user's own version of a shared track is stored", func(t *testing.T) {
		s := newScenario(t)
		mp3 := makeAudio(t, "song.mp3", audioSpec{Tags: songTags})
		s.share(alice, s.uploaded(alice, s.uploadAudioFile(mp3)), "🔗 Трек")
		flac := s.uploadDocument(makeAudio(t, "song.flac", audioSpec{Tags: songTags}), "audio/flac")
		s.botAPI.forget()

		s.send(s.documentMessage(bob, flac))
		s.waitIngest()

		assert.Equal(t, []string{"Artist/Album/01 - Dup Song.flac"}, s.personalFiles(bob))
		assert.Equal(t, []string{"Artist/Album/01 - Dup Song.mp3"}, s.sharedFiles())
		assert.Empty(t, s.botAPI.replies(t))
	})

	t.Run("upload of a track private to another user reveals nothing", func(t *testing.T) {
		s := newScenario(t)
		s.uploaded(alice, s.uploadAudio("track.mp3"))
		s.botAPI.forget()

		s.uploaded(bob, s.uploadAudio("track.mp3"))

		assert.NotEmpty(t, s.botAPI.callsTo("getFile"))
		assert.Equal(t, []string{fixtureTrackPath}, s.personalFiles(bob))
		assert.Empty(t, s.botAPI.replies(t))
	})

	t.Run("send file button sends the audio", func(t *testing.T) {
		s := newScenario(t)
		audio := s.uploadAudio("track.mp3")
		s.share(alice, s.uploaded(alice, audio), "🔗 Трек")
		s.send(s.textMessage(bob, "/shared"))

		s.press(bob, buttonNamed(t, s, "1. ▶️ Прислать файл"))

		sent := s.botAPI.callsTo("sendAudio")
		require.Len(t, sent, 1)
		assert.Equal(t, audio.FileID, sent[0].Params["audio"])
		assert.Equal(t, "1002", sent[0].Params["chat_id"])
	})
}

func TestSharedFeed(t *testing.T) {
	t.Parallel()

	t.Run("inline shared lists recent Shares with their authors", func(t *testing.T) {
		s := newScenario(t)
		aliceAudio := s.uploadAudio("track.mp3")
		s.share(alice, s.uploaded(alice, aliceAudio), "🔗 Трек")
		bobAudio := s.uploadAudioFile(makeAudio(t, "song.mp3", audioSpec{Tags: songTags}))
		s.share(bob, s.uploaded(bob, bobAudio), "🔗 Трек")
		query := s.inlineQuery(alice, "shared")

		s.send(query)

		results := s.botAPI.inlineAnswerTo(t, query).Results
		require.Len(t, results, 3)
		assert.Equal(t, bobAudio.FileID, results[1].AudioFileID)
		assert.Contains(t, results[1].Caption, "@bob")
		assert.Equal(t, aliceAudio.FileID, results[2].AudioFileID)
		assert.Contains(t, results[2].Caption, "@alice")
	})

	t.Run("empty feed says nobody has shared yet", func(t *testing.T) {
		s := newScenario(t)

		s.send(s.textMessage(alice, "/shared"))

		assert.Contains(t, lastReply(t, s).Text, "Пока никто ничего не расшарил")
	})
}

// take presses "take" on the n-th Share of the feed.
func (s *scenario) take(user telegramUser, n int) {
	s.t.Helper()

	s.send(s.textMessage(user, "/shared"))
	s.press(user, buttonNamed(s.t, s, strconv.Itoa(n)+". ➕ Взять себе"))
}

func (s *scenario) feed(user telegramUser) string {
	s.t.Helper()

	s.send(s.textMessage(user, "/shared"))
	return lastReply(s.t, s).Text
}

// botAudio is the audio the bot sent to the user, as the user's client shows it.
func (s *scenario) botAudio(to telegramUser, audio models.Audio) *models.Update {
	update := s.audioMessage(to, audio)
	update.Message.From = &models.User{ID: 123456, IsBot: true, Username: botUsername}
	return update
}

func TestAuthorName(t *testing.T) {
	t.Parallel()

	t.Run("admin from config is shown by username", func(t *testing.T) {
		s := newScenario(t)
		s.share(adminUser, s.uploaded(adminUser, s.uploadAudio("track.mp3")), "🔗 Трек")

		assert.Contains(t, s.feed(alice), "@"+adminUser.Username)
	})

	t.Run("renamed user is shown by the new username", func(t *testing.T) {
		s := newScenario(t)
		s.share(alice, s.uploaded(alice, s.uploadAudio("track.mp3")), "🔗 Трек")
		renamed := telegramUser{ID: alice.ID, Username: "alice_new"}

		s.send(s.textMessage(renamed, "/start"))

		assert.Contains(t, s.feed(bob), "@alice_new")
	})
}
