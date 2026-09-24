package e2e

import (
	"fmt"
	"testing"
	"time"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShare(t *testing.T) {
	t.Run("shared track is found by another user", func(t *testing.T) {
		s := newScenario(t)
		bobAccount := s.linkNewAccount(bob)
		upload := s.uploaded(alice, s.uploadAudio("track.mp3"))

		s.share(alice, upload, "🔗 Трек")

		assert.Equal(t, []string{"Fixture Artist/Fixture Album/01 - Fixture Song.mp3"}, s.sharedFiles())
		env.navidrome.indexedTrack(t, bobAccount, s.library, fixtureTitle)
		assert.Contains(t, lastCallbackAnswer(t, s), "В общей библиотеке")
		assert.Equal(t, []string{"🔒 Снять Share", "🔒 Снять альбом"}, buttonTexts(s.botAPI.Buttons(t)))
	})

	t.Run("shared album puts all its tracks into the Shared Library", func(t *testing.T) {
		s := newScenario(t)
		uploads := s.uploadAlbum(alice, "Album", 3)

		s.share(alice, uploads[0], "💿 Альбом целиком")

		assert.Equal(t, []string{
			"Artist/Album/01 - Song 1.mp3",
			"Artist/Album/02 - Song 2.mp3",
			"Artist/Album/03 - Song 3.mp3",
		}, s.sharedFiles())
		assert.Equal(t, []string{"🔒 Снять Share", "🔒 Снять альбом"}, buttonTexts(s.botAPI.Buttons(t)))
	})

	t.Run("unshared track disappears for others", func(t *testing.T) {
		s := newScenario(t)
		bobAccount := s.linkNewAccount(bob)
		upload := s.uploaded(alice, s.uploadAudio("track.mp3"))
		s.share(alice, upload, "🔗 Трек")
		env.navidrome.indexedTrack(t, bobAccount, s.library, fixtureTitle)

		s.press(alice, buttonNamed(t, s, "🔒 Снять Share"))

		assert.Empty(t, s.sharedFiles())
		assert.Equal(t, []string{"Fixture Artist/Fixture Album/01 - Fixture Song.mp3"}, s.personalFiles(alice))
		env.navidrome.untilGone(t, bobAccount, s.library, fixtureTitle)
		assert.Equal(t, []string{"🔗 Трек", "💿 Альбом целиком"}, buttonTexts(s.botAPI.Buttons(t)))
	})

	t.Run("second sharer learns who shared first", func(t *testing.T) {
		s := newScenario(t)
		mp3 := makeAudio(t, "song.mp3", audioSpec{Tags: songTags})
		s.share(alice, s.uploaded(alice, s.uploadAudioFile(mp3)), "🔗 Трек")
		bobUpload := s.uploaded(bob, s.uploadAudioFile(mp3))

		s.share(bob, bobUpload, "🔗 Трек")

		assert.Contains(t, lastCallbackAnswer(t, s), "уже в общей, расшарил @alice")
		assert.Equal(t, []string{"Artist/Album/01 - Dup Song.mp3"}, s.sharedFiles())
	})

	t.Run("track stays shared while its second sharer keeps it", func(t *testing.T) {
		s := newScenario(t)
		mp3 := makeAudio(t, "song.mp3", audioSpec{Tags: songTags})
		s.share(alice, s.uploaded(alice, s.uploadAudioFile(mp3)), "🔗 Трек")
		aliceButtons := s.botAPI.Buttons(t)
		s.share(bob, s.uploaded(bob, s.uploadAudioFile(mp3)), "🔗 Трек")

		s.press(alice, buttonIn(t, aliceButtons, "🔒 Снять Share"))

		assert.Equal(t, []string{"Artist/Album/01 - Dup Song.mp3"}, s.sharedFiles())
	})

	t.Run("Inbox track cannot be shared", func(t *testing.T) {
		s := newScenario(t)
		upload := s.uploaded(alice, s.uploadAudioFile(makeAudio(t, "audio_1.mp3", audioSpec{})))

		s.send(s.replyCommand(alice, "/share", upload))

		assert.Contains(t, lastReply(t, s).Text, "из Inbox нельзя расшарить")
		assert.Empty(t, s.botAPI.Buttons(t))
		assert.Empty(t, s.sharedFiles())
	})

	t.Run("share without own audio explains how to use it", func(t *testing.T) {
		s := newScenario(t)
		bobUpload := s.uploaded(bob, s.uploadAudio("track.mp3"))

		s.send(s.textMessage(alice, "/share"))
		s.send(s.replyCommand(alice, "/share", bobUpload))

		replies := s.botAPI.Replies(t)
		require.Len(t, replies, 2)
		assert.Contains(t, replies[0].Text, "Ответьте /share")
		assert.Contains(t, replies[1].Text, "Ответьте /share")
		assert.Empty(t, s.botAPI.Buttons(t))
	})
}

// uploaded sends the audio as the user and waits until it is ingested.
func (s *scenario) uploaded(from telegramUser, audio models.Audio) *models.Update {
	s.t.Helper()

	upload := s.audioMessage(from, audio)
	s.send(upload)
	s.waitIngest()
	return upload
}

func (s *scenario) uploadAlbum(from telegramUser, album string, tracks int) []*models.Update {
	s.t.Helper()

	uploads := make([]*models.Update, 0, tracks)
	for n := 1; n <= tracks; n++ {
		tags := map[string]string{
			"artist": "Artist", "album": album, "track": fmt.Sprint(n), "title": fmt.Sprintf("Song %d", n),
		}
		file := makeAudio(s.t, fmt.Sprintf("song%d.mp3", n), audioSpec{Tags: tags})
		uploads = append(uploads, s.uploaded(from, s.uploadAudioFile(file)))
	}
	return uploads
}

func (s *scenario) replyCommand(from telegramUser, command string, to *models.Update) *models.Update {
	return s.message(from, func(m *models.Message) {
		m.Text = command
		m.ReplyToMessage = to.Message
	})
}

// share answers the upload with /share and presses the named button.
func (s *scenario) share(from telegramUser, upload *models.Update, name string) {
	s.t.Helper()

	s.send(s.replyCommand(from, "/share", upload))
	s.press(from, buttonNamed(s.t, s, name))
}

func buttonNamed(t *testing.T, s *scenario, name string) button {
	t.Helper()
	return buttonIn(t, s.botAPI.Buttons(t), name)
}

func buttonIn(t *testing.T, buttons []button, name string) button {
	t.Helper()

	for _, b := range buttons {
		if b.Text == name {
			return b
		}
	}
	t.Fatalf("no button %q among %v", name, buttons)
	return button{}
}

func buttonTexts(buttons []button) []string {
	var texts []string
	for _, b := range buttons {
		texts = append(texts, b.Text)
	}
	return texts
}

func lastCallbackAnswer(t *testing.T, s *scenario) string {
	t.Helper()

	answers := s.botAPI.CallbackAnswers()
	require.NotEmpty(t, answers)
	return answers[len(answers)-1]
}

// untilGone rescans until the account no longer finds the song in libraryDir.
func (n *navidrome) untilGone(t *testing.T, account navidromeAccount, libraryDir string, title string) {
	t.Helper()

	require.Eventually(t, func() bool {
		_ = n.subsonic("startScan", nil, nil)
		songs, err := n.search(account, libraryDir, title)
		return err == nil && len(songs) == 0
	}, time.Minute, 200*time.Millisecond, "%s still finds %q in %s", account.Login, title, libraryDir)
}
