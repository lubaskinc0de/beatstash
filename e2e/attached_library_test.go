package e2e

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/audiofile"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/navidrome"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/telegram"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/zvuk"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/share_tracks"
)

func TestAttachedLibraryAccess(t *testing.T) {
	t.Parallel()

	t.Run("link tells how many songs of the account's libraries the bot sees", func(t *testing.T) {
		s, account, _ := newWithOwnLibrary(t, []string{audiofile.Fixture("track.mp3"), audiofile.Fixture("track.flac")})

		s.Link(alice, account)

		assert.Contains(t, s.WindowText(), s.Catalog(alice).Linked(account.Login, 2))
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

		assert.Contains(t, aliceSees, s.Catalog(alice).Linked(aliceAccount.Login, 1))
		assert.Contains(t, s.WindowText(), s.Catalog(bob).Linked(bobAccount.Login, 1))
	})

	t.Run("Navidrome admin sees every attached library", func(t *testing.T) {
		var account navidrome.Account
		s := harness.NewOwnNavidrome(t, func(s *harness.Scenario) {
			account = s.Navidrome.CreateAdminAccount(t, "alice")
			other := s.NewNavidromeLibrary("other", audiofile.Fixture("track.mp3"))
			s.Navidrome.UntilSongs(t, other.ID, 1)
		})

		s.Link(alice, account)

		assert.Contains(t, s.WindowText(), s.Catalog(alice).LinkedAdmin(account.Login, 1))
	})

	t.Run("library closed to the account is not seen", func(t *testing.T) {
		var account navidrome.Account
		s := harness.NewOwnNavidrome(t, func(s *harness.Scenario) {
			account = s.Navidrome.CreateAccount(t, "alice")
			other := s.NewNavidromeLibrary("other", audiofile.Fixture("track.mp3"))
			s.Navidrome.UntilSongs(t, other.ID, 1)
		})

		s.Link(alice, account)

		c := s.Catalog(alice)
		assert.Equal(t, c.Linked(account.Login, 0)+"\n\n"+c.NavidromeLinked(account.Login), s.WindowText())
	})

	t.Run("library inside music_dir but apart from the bot's folders is attached", func(t *testing.T) {
		var account navidrome.Account
		s := harness.NewOwnNavidrome(t, func(s *harness.Scenario) {
			account = s.Navidrome.CreateAccount(t, "alice")
			own := s.NewMusicDirLibrary("own", audiofile.Fixture("track.mp3"))
			s.Navidrome.OpenLibrary(t, account, own.ID)
			s.Navidrome.UntilSongs(t, own.ID, 1)
		})

		s.Link(alice, account)

		assert.Contains(t, s.WindowText(), s.Catalog(alice).Linked(account.Login, 1))
	})

	t.Run("attached library keeps the admin's default for new accounts", func(t *testing.T) {
		var own harness.NavidromeLibrary
		s := harness.NewOwnNavidrome(t, func(s *harness.Scenario) {
			own = s.NewMusicDirLibrary("own")
			s.Navidrome.GiveToNewAccounts(t, own.ID)
		})

		account := s.Navidrome.CreateAccountWithDefaults(t, "dave")

		assert.Contains(t, s.Navidrome.Libraries(t, account), own.Path)
	})

	t.Run("link takes away a library that holds other users' Personal Libraries", func(t *testing.T) {
		var account navidrome.Account
		s := harness.NewOwnNavidrome(t, func(s *harness.Scenario) {
			account = s.Navidrome.CreateAccount(t, "alice")
			whole := s.NewNavidromeRootLibrary()
			s.Navidrome.OpenLibrary(t, account, whole.ID)
		})

		s.Link(alice, account)

		assert.NotContains(t, s.Navidrome.Libraries(t, account), navidrome.LibraryMount)
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

		c := s.Catalog(alice)
		assert.Equal(t, c.Linked(account.Login, 0)+"\n\n"+c.NavidromeLinked(account.Login), s.WindowText())
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
		s.Share(bob, s.Uploaded(bob, s.UploadAudio("track.mp3")), s.Catalog(bob).ShareTrack())
		s.Open(alice, s.Catalog(alice).FeedButton())

		s.Press(alice, s.Button(s.Catalog(alice).InLibraryButton(1)))

		assert.Equal(t, s.Catalog(alice).AlreadyInLibrary(), s.LastCallbackAnswer())
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

		s.OpenZvuk(alice, s.Catalog(alice).ImportCollection())

		assert.Contains(t, s.WindowText(), s.Catalog(alice).AllImported("zvuk", 1))
	})
}

func TestShareFromAttachedLibrary(t *testing.T) {
	t.Parallel()

	t.Run("np shares a song of the attached library", func(t *testing.T) {
		s, account, own := newWithOwnLibrary(t, []string{audiofile.Fixture("track.mp3")})
		s.Link(alice, account)
		before := harness.FilesWithContent(t, own.Dir)
		s.Navidrome.StartPlaying(t, account, s.Navidrome.IndexedTrack(t, account, own.Dir, audiofile.FixtureTitle).ID)
		share := telegram.ButtonNamed(t, s.NowPlaying(alice).Buttons(), s.Catalog(alice).ShareButton())

		s.PressInline(alice, share)

		assert.Contains(t, s.LastCallbackAnswer(), s.Catalog(alice).ShareResult(&share_tracks.ShareResult{Created: 1}))
		assert.Equal(t, []string{audiofile.FixtureTrackPath}, s.SharedFiles())
		assert.Equal(t, before, harness.FilesWithContent(t, own.Dir))
		s.Open(bob, s.Catalog(bob).FeedButton())
		assert.Contains(t, s.WindowText(), audiofile.FixtureTitle)
	})

	t.Run("song over the Shared Library's Quota is not shared", func(t *testing.T) {
		s, account, own := newWithOwnLibrary(t, []string{audiofile.Fixture("track.mp3")}, harness.WithSharedQuota(20*kb))
		s.Link(alice, account)
		s.Navidrome.StartPlaying(t, account, s.Navidrome.IndexedTrack(t, account, own.Dir, audiofile.FixtureTitle).ID)
		share := telegram.ButtonNamed(t, s.NowPlaying(alice).Buttons(), s.Catalog(alice).ShareButton())

		s.PressInline(alice, share)

		assert.Equal(t, s.Catalog(alice).SharedLibraryFull(""), s.LastCallbackAnswer())
		assert.Empty(t, s.SharedFiles())
	})
}

func TestAttachedLibraryRefresh(t *testing.T) {
	t.Parallel()

	t.Run("song added to the library is seen after a while", func(t *testing.T) {
		s, account, own := newWithOwnLibrary(t, []string{audiofile.Fixture("track.mp3")}, harness.WithAttachInterval(100*time.Millisecond))

		s.AddToNavidromeLibrary(own, audiofile.Fixture("track.flac"), "track.flac")
		s.Navidrome.UntilSongs(t, own.ID, 2)
		s.WaitAttach()

		s.Link(alice, account)
		assert.Contains(t, s.WindowText(), s.Catalog(alice).Linked(account.Login, 2))
	})

	t.Run("gone song leaves the bot and its Share stays in the feed", func(t *testing.T) {
		s, account, own := newWithOwnLibrary(t,
			[]string{audiofile.Fixture("track.mp3"), audiofile.Fixture("track.flac")},
			harness.WithAttachInterval(100*time.Millisecond),
		)
		s.Link(alice, account)
		s.Navidrome.StartPlaying(t, account, s.Navidrome.SongAt(t, account, audiofile.FixtureTitle, filepath.Join(own.Dir, "track.mp3")).ID)
		s.PressInline(alice, telegram.ButtonNamed(t, s.NowPlaying(alice).Buttons(), s.Catalog(alice).ShareButton()))

		require.NoError(t, os.Remove(filepath.Join(own.Dir, "track.mp3")))
		s.Navidrome.UntilSongs(t, own.ID, 1)
		s.WaitAttach()

		s.Link(alice, account)
		assert.Contains(t, s.WindowText(), s.Catalog(alice).Linked(account.Login, 1))
		assert.Equal(t, []string{audiofile.FixtureTrackPath}, s.SharedFiles())
		s.Open(bob, s.Catalog(bob).FeedButton())
		assert.Contains(t, s.WindowText(), audiofile.FixtureTitle)
	})
}

func TestAttachedLibraryChanges(t *testing.T) {
	t.Parallel()

	t.Run("moving an attached library with the same Navidrome ID keeps it attached", func(t *testing.T) {
		var account navidrome.Account
		var own harness.NavidromeLibrary
		s := harness.NewOwnNavidrome(t, func(s *harness.Scenario) {
			account = s.Navidrome.CreateAccount(t, "alice")
			own = s.NewNavidromeLibrary("own", audiofile.Fixture("track.mp3"))
			s.Navidrome.OpenLibrary(t, account, own.ID)
			s.Navidrome.UntilSongs(t, own.ID, 1)
		}, harness.WithAttachInterval(time.Hour))
		s.Link(alice, account)
		moved := s.NewNavidromeLibrary("moved", audiofile.Fixture("track.mp3"))
		s.Navidrome.DeleteLibrary(t, moved.ID)
		s.Navidrome.MoveLibrary(t, own.ID, moved.Path)
		s.Restart()
		s.Navidrome.UntilSongs(t, own.ID, 1)
		track := s.Navidrome.IndexedTrack(t, account, moved.Path, audiofile.FixtureTitle)
		s.Navidrome.StartPlaying(t, account, track.ID)

		share := telegram.ButtonNamed(t, s.NowPlaying(alice).Buttons(), s.Catalog(alice).ShareButton())
		s.PressInline(alice, share)

		assert.Contains(t, s.LastCallbackAnswer(), s.Catalog(alice).ShareResult(&share_tracks.ShareResult{Created: 1}))
		assert.Equal(t, []string{audiofile.FixtureTrackPath}, s.SharedFiles())
	})

	t.Run("replacing a deleted library at the same path attaches the replacement", func(t *testing.T) {
		var account navidrome.Account
		var own harness.NavidromeLibrary
		s := harness.NewOwnNavidrome(t, func(s *harness.Scenario) {
			account = s.Navidrome.CreateAccount(t, "alice")
			own = s.NewNavidromeLibrary("own", audiofile.Fixture("track.mp3"))
			s.Navidrome.OpenLibrary(t, account, own.ID)
			s.Navidrome.UntilSongs(t, own.ID, 1)
		})
		s.Link(alice, account)

		s.Navidrome.DeleteLibrary(t, own.ID)
		replacementID := s.Navidrome.CreateLibrary(t, "replacement", own.Path)
		s.Navidrome.OpenLibrary(t, account, replacementID)
		s.Navidrome.UntilSongs(t, replacementID, 1)
		s.Restart()
		s.Link(alice, account)

		assert.Contains(t, s.WindowText(), s.Catalog(alice).Linked(account.Login, 1))
		track := s.Navidrome.IndexedTrack(t, account, own.Path, audiofile.FixtureTitle)
		s.Navidrome.StartPlaying(t, account, track.ID)
		share := telegram.ButtonNamed(t, s.NowPlaying(alice).Buttons(), s.Catalog(alice).ShareButton())
		s.PressInline(alice, share)
		assert.Contains(t, s.LastCallbackAnswer(), s.Catalog(alice).ShareResult(&share_tracks.ShareResult{Created: 1}))
	})
}
