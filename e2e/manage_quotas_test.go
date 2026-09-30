package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/audiofile"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

const (
	mb = 1024 * kb
	// smallQuota is what the Admin types as "20 KB": the fixture track does not fit.
	smallQuota = 20 * kb
)

func TestServerQuotas(t *testing.T) {
	t.Parallel()

	t.Run("new Default Quota limits the uploads of users without their own at once", func(t *testing.T) {
		s := harness.New(t)

		setDefaultQuota(s, "20 КБ")

		assertUploadRefused(t, s, alice)
	})

	t.Run("after a restart the Admin's Default Quota holds, not the config's", func(t *testing.T) {
		s := harness.New(t, harness.WithDefaultQuota(mb))
		setDefaultQuota(s, "20 KB")

		s.Restart()

		assertUploadRefused(t, s, alice)
	})

	t.Run("reset brings back the config's Default Quota", func(t *testing.T) {
		s := harness.New(t, harness.WithDefaultQuota(smallQuota))
		c := s.Catalog(admin)
		s.Open(admin, c.AdminButton(), c.QuotasButton(), c.EditDefaultQuota(), c.UnlimitedButton())

		s.Go(admin, c.EditDefaultQuota(), c.FromConfigButton())

		assertUploadRefused(t, s, alice)
	})

	t.Run("unreadable size changes nothing and the bot still waits for one", func(t *testing.T) {
		s := harness.New(t, harness.WithDefaultQuota(smallQuota))
		setDefaultQuota(s, "a lot")
		assert.Contains(t, s.WindowText(), s.Catalog(admin).QuotaUnreadable())
		assertUploadRefused(t, s, alice)

		s.SendText(admin, "1.5 MB")

		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		assert.Equal(t, []string{audiofile.FixtureTrackPath}, s.PersonalFiles(alice))
	})

	t.Run("Shared Library's Quota set by the Admin refuses a Share", func(t *testing.T) {
		s := harness.New(t)
		c := s.Catalog(admin)
		s.Open(admin, c.AdminButton(), c.QuotasButton(), c.EditSharedQuota())
		s.SendText(admin, "20 KB")

		s.Share(alice, s.Uploaded(alice, s.UploadAudio("track.mp3")), s.Catalog(alice).ShareTrack())

		assert.Equal(t, s.Catalog(alice).SharedLibraryFull(""), s.LastCallbackAnswer())
		assert.Empty(t, s.SharedFiles())
	})
}

func TestUserQuota(t *testing.T) {
	t.Parallel()

	t.Run("user with an own Quota keeps it when the Default Quota changes", func(t *testing.T) {
		s := harness.New(t)
		setUserQuota(t, s, alice, "1 MB")

		setDefaultQuota(s, "20 KB")

		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		assert.Equal(t, []string{audiofile.FixtureTrackPath}, s.PersonalFiles(alice))
		assertUploadRefused(t, s, bob)
	})

	t.Run("default returns the user to the Default Quota", func(t *testing.T) {
		s := harness.New(t, harness.WithDefaultQuota(smallQuota))
		setUserQuota(t, s, alice, "1 MB")

		s.Go(admin, s.Catalog(admin).EditUserQuota(), s.Catalog(admin).DefaultQuotaButton())

		assertUploadRefused(t, s, alice)
	})

	t.Run("Quota below the Usage keeps the Tracks and refuses new ones", func(t *testing.T) {
		s := harness.New(t)
		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		used := harness.FileSize(t, s.PersonalPath(alice, audiofile.FixtureTrackPath))

		setUserQuota(t, s, alice, "1 KB")

		assert.Contains(t, s.WindowText(), s.Catalog(admin).CardUsage(1, library.Usage{Used: used, Quota: kb}, true))
		upload := s.Uploaded(alice, s.UploadAudioFile(audiofile.Generate(t, "other.mp3", audiofile.Spec{Tags: audiofile.SongTags})))
		assert.Equal(t, s.Catalog(alice).NoRoom(library.Usage{Used: used, Quota: kb}, ""), s.LastReply().Text)
		assert.Equal(t, upload.Message.ID, s.LastReply().ReplyTo)
		assert.Equal(t, []string{audiofile.FixtureTrackPath}, s.PersonalFiles(alice))
	})
}

func TestCapacity(t *testing.T) {
	t.Parallel()

	t.Run("promise sums own Quotas, the Default Quota of the rest and the Shared Library's", func(t *testing.T) {
		s := harness.New(t, harness.WithDefaultQuota(10*mb), harness.WithSharedQuota(5*mb))
		setUserQuota(t, s, alice, "1 MB")

		s.Open(admin, s.Catalog(admin).AdminButton())

		text := s.WindowText()
		assert.Contains(t, text, s.Catalog(admin).Promised(26*mb))
		assert.NotContains(t, text, s.Catalog(admin).OvercommitWarning())
	})

	t.Run("any unlimited Quota promises everything and warns", func(t *testing.T) {
		s := harness.New(t, harness.WithDefaultQuota(10*mb))

		s.Open(admin, s.Catalog(admin).AdminButton(), s.Catalog(admin).QuotasButton())

		text := s.WindowText()
		assert.Contains(t, text, s.Catalog(admin).Promised(library.Unlimited))
		assert.Contains(t, text, s.Catalog(admin).OvercommitWarning())
	})

	t.Run("promise beyond the disk warns", func(t *testing.T) {
		s := harness.New(t, harness.WithDefaultQuota(1024*1024*1024*mb), harness.WithSharedQuota(mb))

		s.Open(admin, s.Catalog(admin).AdminButton())

		assert.Contains(t, s.WindowText(), s.Catalog(admin).OvercommitWarning())
	})
}

func setDefaultQuota(s *harness.Scenario, size string) {
	c := s.Catalog(admin)
	s.Open(admin, c.AdminButton(), c.QuotasButton(), c.EditDefaultQuota())
	s.SendText(admin, size)
}

// setUserQuota leaves the Admin on the user's card.
func setUserQuota(t *testing.T, s *harness.Scenario, user harness.User, size string) {
	t.Helper()

	openCard(t, s, user)
	s.Go(admin, s.Catalog(admin).EditUserQuota())
	s.SendText(admin, size)
}

// assertUploadRefused expects the user to follow smallQuota and keep nothing.
func assertUploadRefused(t *testing.T, s *harness.Scenario, user harness.User) {
	t.Helper()

	upload := s.Uploaded(user, s.UploadAudio("track.mp3"))

	require.Empty(t, s.PersonalFiles(user))
	reply := s.LastReply()
	assert.Equal(t, upload.Message.ID, reply.ReplyTo)
	assert.Equal(t, s.Catalog(user).NoRoom(library.Usage{Quota: smallQuota}, ""), reply.Text)
}
