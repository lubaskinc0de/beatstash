package e2e

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/audiofile"
)

func TestInvite(t *testing.T) {
	t.Parallel()

	t.Run("invited person becomes a user", func(t *testing.T) {
		s := harness.New(t)
		carol := harness.Newcomer("carol")
		code := s.Invite()

		s.Send(s.TextMessage(carol, "/start "+code))
		audio := s.UploadAudio("track.mp3")
		upload := s.AudioMessage(carol, audio)
		s.Send(upload)
		s.WaitIngest()

		assert.Contains(t, s.Telegram.Replies(t)[1].Text, "Добро пожаловать")
		assert.Equal(t, []string{"👀", "👍"}, s.Telegram.ReactionsOn(t, upload.Message.ID))
	})

	t.Run("newcomer's library is named by their user id", func(t *testing.T) {
		s := harness.New(t)
		carol := harness.Newcomer("carol")
		s.Send(s.TextMessage(carol, "/start "+s.Invite()))

		s.Uploaded(carol, s.UploadAudio("track.mp3"))

		assert.Equal(t, []string{filepath.Join("users", "4", audiofile.FixtureTrackPath)}, s.LibraryFiles())
	})

	t.Run("used invite is rejected", func(t *testing.T) {
		s := harness.New(t)
		code := s.Invite()
		s.Send(s.TextMessage(harness.Newcomer("carol"), "/start "+code))
		dave := harness.Newcomer("dave")

		s.Send(s.TextMessage(dave, "/start "+code))
		upload := s.AudioMessage(dave, s.UploadAudio("track.mp3"))
		s.Send(upload)

		assert.Contains(t, s.LastReply().Text, "Приглашение недействительно")
		assert.Empty(t, s.Telegram.ReactionsOn(t, upload.Message.ID))
	})

	t.Run("invite expires after 7 days", func(t *testing.T) {
		s := harness.New(t)
		code := s.Invite()
		s.Clock.Advance(7*24*time.Hour + time.Minute)
		carol := harness.Newcomer("carol")

		s.Send(s.TextMessage(carol, "/start "+code))
		upload := s.AudioMessage(carol, s.UploadAudio("track.mp3"))
		s.Send(upload)

		assert.Contains(t, s.LastReply().Text, "Приглашение недействительно")
		assert.Empty(t, s.Telegram.ReactionsOn(t, upload.Message.ID))
	})

	t.Run("non-admin cannot invite", func(t *testing.T) {
		s := harness.New(t)

		s.Send(s.TextMessage(alice, "/invite"))

		replies := s.Telegram.Replies(t)
		require.Len(t, replies, 1)
		assert.Contains(t, replies[0].Text, "только администратор")
		assert.NotContains(t, replies[0].Text, "?start=")
	})

	t.Run("admin dropped from config can no longer invite", func(t *testing.T) {
		s := harness.New(t)

		s.Restart(harness.WithAdmins(alice))
		s.Send(s.TextMessage(admin, "/invite"))
		refused := s.LastReply().Text
		s.Send(s.TextMessage(alice, "/invite"))

		assert.Contains(t, refused, "только администратор")
		assert.Contains(t, s.LastReply().Text, "?start=")
	})

	t.Run("stranger gets no answer but to start", func(t *testing.T) {
		s := harness.New(t)

		s.Send(s.TextMessage(stranger, "/invite"))
		s.Send(s.TextMessage(stranger, "/share"))
		s.Send(s.TextMessage(stranger, "/top"))
		s.Send(s.TextMessage(stranger, "hello"))
		s.Send(s.AudioMessage(stranger, s.UploadAudio("track.mp3")))

		assert.Empty(t, s.Telegram.AllCalls())
	})
}
