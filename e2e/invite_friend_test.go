package e2e

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/lubaskinc0de/beatstash/e2e/harness"
	"github.com/lubaskinc0de/beatstash/e2e/harness/audiofile"
	"github.com/lubaskinc0de/beatstash/e2e/harness/telegram"
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

		assert.Contains(t, s.WindowText(), s.Catalog(dave).InviteInvalid())
		assert.Contains(t, s.WindowText(), s.Catalog(dave).StrangerHome(4, 0, ""))
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

		assert.Contains(t, s.WindowText(), s.Catalog(carol).InviteInvalid())
		assert.Empty(t, s.Telegram.ReactionsOn(t, upload.Message.ID))
	})

	t.Run("admin gets another working invite", func(t *testing.T) {
		s := harness.New(t)
		s.Open(admin, s.Catalog(admin).AdminButton(), s.Catalog(admin).InviteButton())
		first := s.InviteCode()

		s.Go(admin, s.Catalog(admin).AnotherInvite())
		second := s.InviteCode()
		s.Register(harness.Newcomer("carol"))
		dave := harness.Newcomer("dave")
		s.Send(s.TextMessage(dave, "/start "+second))

		assert.NotEqual(t, first, second)
		assert.Contains(t, s.WindowText(), s.Catalog(dave).ChooseLogin())
	})

	t.Run("invite says how long it lasts", func(t *testing.T) {
		s := harness.New(t)

		s.Open(admin, s.Catalog(admin).AdminButton(), s.Catalog(admin).InviteButton())

		link := "https://t.me/" + telegram.BotUsername + "?start=" + s.InviteCode()
		assert.Contains(t, s.WindowText(), s.Catalog(admin).Invite(link, 7*24*time.Hour))
	})

	t.Run("admin dropped from config can no longer invite", func(t *testing.T) {
		s := harness.New(t)

		s.Restart(harness.WithAdmins(alice))
		s.Open(admin)
		adminButtons := telegram.ButtonTexts(s.Telegram.Buttons(t))
		s.Open(alice, s.Catalog(alice).AdminButton(), s.Catalog(alice).InviteButton())

		assert.NotContains(t, adminButtons, s.Catalog(admin).AdminButton())
		assert.Contains(t, s.WindowText(), "?start=")
	})

	t.Run("invite command is gone", func(t *testing.T) {
		s := harness.New(t)

		invite := s.TextMessage(admin, "/invite")
		s.Send(invite)

		assertOnlyDeleted(t, s, invite)
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
