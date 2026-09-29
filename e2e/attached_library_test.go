package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/audiofile"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/navidrome"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/telegram"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/zvuk"
)

func TestAttachedLibraryAccess(t *testing.T) {
	t.Parallel()

	t.Run("link tells how many songs of the account's libraries the bot sees", func(t *testing.T) {
		s, account, _ := newWithOwnLibrary(t, []string{audiofile.Fixture("track.mp3"), audiofile.Fixture("track.flac")})

		s.Link(alice, account)

		assert.Contains(t, s.WindowText(), "Бот видит 2 песни из ваших библиотек Navidrome")
	})

	t.Run("library open to two accounts is seen by both", func(t *testing.T) {
		var aliceAccount, bobAccount navidrome.Account
		s := harness.NewOwnNavidrome(t, func(s *harness.Scenario) {
			aliceAccount = s.Navidrome.CreateAccount(t, "alice")
			bobAccount = s.Navidrome.CreateAccount(t, "bob")
			family := s.NewNavidromeLibrary("family", audiofile.Fixture("track.mp3"))
			s.Navidrome.OpenLibrary(t, aliceAccount, family.ID)
			s.Navidrome.OpenLibrary(t, bobAccount, family.ID)
			s.Navidrome.UntilSongs(t, family.ID, 1)
		})

		s.Link(alice, aliceAccount)
		aliceSees := s.WindowText()
		s.Link(bob, bobAccount)

		assert.Contains(t, aliceSees, "Бот видит 1 песню")
		assert.Contains(t, s.WindowText(), "Бот видит 1 песню")
	})

	t.Run("Navidrome admin sees every attached library", func(t *testing.T) {
		var account navidrome.Account
		s := harness.NewOwnNavidrome(t, func(s *harness.Scenario) {
			account = s.Navidrome.CreateAdminAccount(t, "alice")
			other := s.NewNavidromeLibrary("other", audiofile.Fixture("track.mp3"))
			s.Navidrome.UntilSongs(t, other.ID, 1)
		})

		s.Link(alice, account)

		assert.Contains(t, s.WindowText(), "Это администратор Navidrome")
		assert.Contains(t, s.WindowText(), "Бот видит 1 песню")
	})

	t.Run("library closed to the account is not seen", func(t *testing.T) {
		var account navidrome.Account
		s := harness.NewOwnNavidrome(t, func(s *harness.Scenario) {
			account = s.Navidrome.CreateAccount(t, "alice")
			other := s.NewNavidromeLibrary("other", audiofile.Fixture("track.mp3"))
			s.Navidrome.UntilSongs(t, other.ID, 1)
		})

		s.Link(alice, account)

		assert.Contains(t, s.WindowText(), "привязан")
		assert.NotContains(t, s.WindowText(), "Бот видит")
	})

	t.Run("library spanning music_dir is not attached", func(t *testing.T) {
		var account navidrome.Account
		s := harness.NewOwnNavidrome(t, func(s *harness.Scenario) {
			account = s.Navidrome.CreateAccount(t, "alice")
			whole := s.NewNavidromeRootLibrary(audiofile.Fixture("track.mp3"))
			s.Navidrome.OpenLibrary(t, account, whole.ID)
			s.Navidrome.UntilSongs(t, whole.ID, 1)
		})

		s.Link(alice, account)

		assert.Contains(t, s.WindowText(), "привязан")
		assert.NotContains(t, s.WindowText(), "Бот видит")
	})
}

// newWithOwnLibrary gives Alice's account a Navidrome library of its own
// that holds the files; the account is not linked yet.
func newWithOwnLibrary(
	t *testing.T,
	files []string,
	opts ...harness.Option,
) (*harness.Scenario, navidrome.Account, harness.NavidromeLibrary) {
	t.Helper()

	var account navidrome.Account
	var own harness.NavidromeLibrary
	s := harness.NewOwnNavidrome(t, func(s *harness.Scenario) {
		account = s.Navidrome.CreateAccount(t, "alice")
		own = s.NewNavidromeLibrary("own", files...)
		s.Navidrome.OpenLibrary(t, account, own.ID)
		s.Navidrome.UntilSongs(t, own.ID, len(files))
	}, opts...)
	return s, account, own
}

// keptSong is the Zvuk track "Zvuk Band — Kept Song" of AddZvukSong as the
// user keeps it in their own Navidrome library.
func keptSong(t *testing.T) string {
	t.Helper()
	return audiofile.Generate(t, "kept.mp3", audiofile.Spec{Seconds: 2, Bitrate: "128k", Tags: map[string]string{
		"artist": "Zvuk Band", "album_artist": "Zvuk Band", "album": "Zvuk Album", "title": "Kept Song", "track": "1",
	}})
}

func TestAttachedLibraryDuplicates(t *testing.T) {
	t.Parallel()

	t.Run("Import skips a track the attached library has and marks its song", func(t *testing.T) {
		s, account, own := newWithOwnLibrary(t, []string{keptSong(t)})
		s.Link(alice, account)
		before := harness.FilesWithContent(t, own.Dir)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukSong("901", "Kept Song", true)
		s.LikeOnZvuk("901")
		s.Zvuk.AddPlaylist(zvuk.Playlist{ID: "811", Title: "Kept", Tracks: []string{"901"}})
		s.Zvuk.Update(harness.ZvukToken, func(a *zvuk.Account) { a.Playlists = []string{"811"} })

		s.ImportZvuk(alice)
		s.WaitIngest()

		assert.Zero(t, s.Zvuk.DownloadsOf("901"))
		assert.Empty(t, s.PersonalFiles(alice))
		s.Navidrome.UntilStarred(t, account, []string{"Zvuk Album/Kept Song"})
		s.Navidrome.UntilPlaylist(t, account, "Kept", []string{"Zvuk Album/Kept Song"})
		assert.Equal(t, before, harness.FilesWithContent(t, own.Dir))
	})

	t.Run("Take of a track the attached library has says it is there", func(t *testing.T) {
		s, account, _ := newWithOwnLibrary(t, []string{audiofile.Fixture("track.mp3")})
		s.Link(alice, account)
		s.Share(bob, s.Uploaded(bob, s.UploadAudio("track.mp3")), "🔗 Трек")
		s.Open(alice, "🎵 Лента")

		s.Press(alice, s.Button("1. ✅ Уже у вас"))

		assert.Equal(t, "Этот трек уже есть у вас", s.LastCallbackAnswer())
		assert.Empty(t, s.PersonalFiles(alice))
	})

	t.Run("repeated Import counts the track the attached library has as stored", func(t *testing.T) {
		s, account, _ := newWithOwnLibrary(t, []string{keptSong(t)})
		s.Link(alice, account)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukSong("901", "Kept Song", true)
		s.LikeOnZvuk("901")
		s.ImportZvuk(alice)
		s.WaitIngest()

		s.OpenZvuk(alice, "📥 Импортировать")

		assert.Contains(t, s.WindowText(), "уже в вашей библиотеке")
	})
}

func TestShareFromAttachedLibrary(t *testing.T) {
	t.Parallel()

	t.Run("np shares a song of the attached library", func(t *testing.T) {
		s, account, own := newWithOwnLibrary(t, []string{audiofile.Fixture("track.mp3")})
		s.Link(alice, account)
		before := harness.FilesWithContent(t, own.Dir)
		s.Navidrome.StartPlaying(t, account, s.Navidrome.IndexedTrack(t, account, own.Dir, audiofile.FixtureTitle).ID)
		share := telegram.ButtonNamed(t, s.NowPlayingButtons(alice), "🔗 Поделиться")

		s.PressInline(alice, share)

		assert.Contains(t, s.LastCallbackAnswer(), "В общей библиотеке")
		assert.Equal(t, []string{audiofile.FixtureTrackPath}, s.SharedFiles())
		assert.Equal(t, before, harness.FilesWithContent(t, own.Dir))
		s.Open(bob, "🎵 Лента")
		assert.Contains(t, s.WindowText(), audiofile.FixtureTitle)
	})
}

func TestAttachedLibraryRefresh(t *testing.T) {
	t.Parallel()

	t.Run("song added to the library is seen after a while", func(t *testing.T) {
		s, account, own := newWithOwnLibrary(t, []string{audiofile.Fixture("track.mp3")}, harness.WithAttachInterval(100*time.Millisecond))

		s.AddToNavidromeLibrary(own, audiofile.Fixture("track.flac"), "track.flac")
		s.Navidrome.UntilSongs(t, own.ID, 2)

		untilLinkSays(t, s, account, "Бот видит 2 песни")
	})

	t.Run("gone song leaves the bot and its Share stays in the feed", func(t *testing.T) {
		s, account, own := newWithOwnLibrary(t,
			[]string{audiofile.Fixture("track.mp3"), audiofile.Fixture("track.flac")},
			harness.WithAttachInterval(100*time.Millisecond),
		)
		s.Link(alice, account)
		s.Navidrome.StartPlaying(t, account, s.Navidrome.SongAt(t, account, audiofile.FixtureTitle, filepath.Join(own.Dir, "track.mp3")).ID)
		s.PressInline(alice, telegram.ButtonNamed(t, s.NowPlayingButtons(alice), "🔗 Поделиться"))

		require.NoError(t, os.Remove(filepath.Join(own.Dir, "track.mp3")))
		s.Navidrome.UntilSongs(t, own.ID, 1)

		untilLinkSays(t, s, account, "Бот видит 1 песню")
		assert.Equal(t, []string{audiofile.FixtureTrackPath}, s.SharedFiles())
		s.Open(bob, "🎵 Лента")
		assert.Contains(t, s.WindowText(), audiofile.FixtureTitle)
	})
}

// untilLinkSays links the account again until the bot's answer has text.
func untilLinkSays(t *testing.T, s *harness.Scenario, account navidrome.Account, text string) {
	t.Helper()

	var got string
	require.Eventually(t, func() bool {
		s.Link(alice, account)
		got = s.WindowText()
		return strings.Contains(got, text)
	}, 30*time.Second, 200*time.Millisecond, "window: %q", &got)
}
