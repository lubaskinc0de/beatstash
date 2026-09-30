package e2e

import (
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/audiofile"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/telegram"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/oversee_service"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

func TestAdminScreen(t *testing.T) {
	t.Parallel()

	t.Run("Admin sees how the service lives", func(t *testing.T) {
		s := harness.New(t)

		s.Open(admin, s.Catalog(admin).AdminButton())

		stats := oversee_service.ServiceStats{Users: 3, ActiveUsers: 3}
		assert.Contains(t, s.WindowText(), s.Catalog(admin).ServiceStats(stats))
	})

	t.Run("user has neither the button nor the screen", func(t *testing.T) {
		s := harness.New(t)
		s.Open(admin)
		adminButton := s.Button(s.Catalog(admin).AdminButton())
		s.Open(alice)
		home := s.Telegram.Window()

		assert.NotContains(t, telegram.ButtonTexts(s.Telegram.Buttons(t)), s.Catalog(alice).AdminButton())
		s.PressOn(alice, adminButton, home.MessageID)
		assert.Contains(t, s.WindowText(), s.Catalog(alice).AdminOnly())
		assert.NotContains(t, s.WindowText(), s.Catalog(alice).ServiceStats(oversee_service.ServiceStats{Users: 3, ActiveUsers: 3}))
	})

	t.Run("stats count the weight, Shared Tracks and Takes", func(t *testing.T) {
		s := harness.New(t)
		s.Share(alice, s.Uploaded(alice, s.UploadAudio("track.mp3")), s.Catalog(alice).ShareTrack())
		s.Take(bob, 1)

		s.Open(admin, s.Catalog(admin).AdminButton())

		used := harness.FileSize(t, s.PersonalPath(alice, audiofile.FixtureTrackPath)) * 3
		stats := oversee_service.ServiceStats{Users: 3, ActiveUsers: 3, Tracks: 3, SharedTracks: 1, Takes: 1}
		text := s.WindowText()
		assert.Contains(t, text, s.Catalog(admin).ServiceStats(stats))
		assert.Contains(t, text, s.Catalog(admin).CapacityUsed(used))
	})

	t.Run("inline query makes a user active", func(t *testing.T) {
		s := harness.New(t)
		s.Clock.Advance(8 * 24 * time.Hour)

		s.Send(s.InlineQuery(alice, "np"))

		s.Open(admin, s.Catalog(admin).AdminButton())
		stats := oversee_service.ServiceStats{Users: 3, ActiveUsers: 2}
		assert.Contains(t, s.WindowText(), s.Catalog(admin).ServiceStats(stats))
	})

	t.Run("stranger who wrote /start counts apart from the users", func(t *testing.T) {
		s := harness.New(t)
		s.Send(s.TextMessage(stranger, "/start"))

		s.Open(admin, s.Catalog(admin).AdminButton())

		stats := oversee_service.ServiceStats{Users: 3, ActiveUsers: 3, Strangers: 1}
		assert.Contains(t, s.WindowText(), s.Catalog(admin).ServiceStats(stats))
	})
}

func TestUserList(t *testing.T) {
	t.Parallel()

	t.Run("users go by Last Seen, 10 a page", func(t *testing.T) {
		s := harness.New(t)
		var newcomers []harness.User
		for range 8 {
			newcomer := harness.Newcomer("user")
			code := s.Invite()
			s.Clock.Advance(time.Minute)
			s.Join(newcomer, code)
			newcomers = append([]harness.User{newcomer}, newcomers...)
		}
		s.Clock.Advance(time.Minute)

		c := s.Catalog(admin)
		s.Open(admin, c.AdminButton(), c.UsersButton())
		first := usernames(s.Telegram.Buttons(t))
		s.Go(admin, c.NextPage())
		second := usernames(s.Telegram.Buttons(t))

		want := []string{admin.Username}
		for _, newcomer := range newcomers {
			want = append(want, newcomer.Username)
		}
		assert.Equal(t, append(want, alice.Username), first)
		assert.Equal(t, []string{bob.Username}, second)
	})

	t.Run("card shows the Tracks and the Usage of the user", func(t *testing.T) {
		s := harness.New(t)
		s.Uploaded(alice, s.UploadAudio("track.mp3"))

		openCard(t, s, alice)

		used := harness.FileSize(t, s.PersonalPath(alice, audiofile.FixtureTrackPath))
		text := s.WindowText()
		assert.Contains(t, text, s.Catalog(admin).CardUsage(1, library.Usage{Used: used}, false))
		assert.Contains(t, text, "tg://user?id="+strconv.FormatInt(alice.ID, 10))
	})

	t.Run("new name in Telegram shows in the card after the user's next request", func(t *testing.T) {
		s := harness.New(t)
		renamed := alice
		renamed.FirstName, renamed.LastName = "Alice", "Liddell"

		s.Open(renamed)

		openCard(t, s, alice)
		assert.Contains(t, s.WindowText(), "Alice Liddell")
	})
}

var username = regexp.MustCompile(`@([\w.-]+)`)

// usernames lists the users the buttons name, in order.
func usernames(buttons []telegram.Button) []string {
	var names []string
	for _, b := range buttons {
		if match := username.FindStringSubmatch(b.Text); match != nil {
			names = append(names, match[1])
		}
	}
	return names
}

func openCard(t *testing.T, s *harness.Scenario, user harness.User) {
	t.Helper()

	s.Open(admin, s.Catalog(admin).AdminButton(), s.Catalog(admin).UsersButton())
	for _, b := range s.Telegram.Buttons(t) {
		if match := username.FindStringSubmatch(b.Text); match != nil && match[1] == user.Username {
			s.Press(admin, b)
			return
		}
	}
	require.FailNow(t, "no button of "+user.Username)
}
