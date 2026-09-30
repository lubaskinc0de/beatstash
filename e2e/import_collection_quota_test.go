package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/audiofile"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/zvuk"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/import_collection"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

// zvukTrackSize is about what a track of AddZvukAlbum weighs in the Library.
func zvukTrackSize(t *testing.T) int64 {
	t.Helper()
	return harness.FileSize(t, audiofile.Generate(t, "high.mp3", audiofile.Spec{Seconds: 2, Bitrate: "320k"}))
}

// twoZvukTracks fits two tracks of AddZvukAlbum but not three.
func twoZvukTracks(t *testing.T) library.Quota {
	t.Helper()
	return library.Quota(zvukTrackSize(t) * 5 / 2)
}

func TestImportQuota(t *testing.T) {
	t.Parallel()

	t.Run("plan tells what the Quota leaves", func(t *testing.T) {
		s := harness.New(t, harness.WithDefaultQuota(100*kb))
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)

		s.OpenZvuk(alice, s.Catalog(alice).ImportCollection())

		plan := &import_collection.Plan{Total: 9, Missing: 9, MissingBytes: 9 * zvukTrackGuess, Usage: library.Usage{Quota: 100 * kb}}
		assert.Contains(t, s.WindowText(), s.Catalog(alice).Plan("zvuk", plan))
	})

	t.Run("Import into a full Personal Library is refused before any download", func(t *testing.T) {
		s := harness.New(t, harness.WithAdminContact(adminContact))
		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		used := harness.FileSize(t, s.PersonalPath(alice, audiofile.FixtureTrackPath))
		s.Restart(harness.WithDefaultQuota(kb))
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)

		s.ImportZvuk(alice)
		s.WaitIngest()

		assert.Equal(t, s.Catalog(alice).NoRoom(library.Usage{Used: used, Quota: kb}, adminContact), s.LastCallbackAnswer())
		assert.Empty(t, s.Zvuk.EventsOf(zvuk.StreamAsked))
		assert.Equal(t, []string{audiofile.FixtureTrackPath}, s.PersonalFiles(alice))
	})

	t.Run("Import over the Quota sends one summary counting what did not fit", func(t *testing.T) {
		s := harness.New(t, harness.WithDefaultQuota(twoZvukTracks(t)), harness.WithAdminContact(adminContact))
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)

		s.ImportZvuk(alice)
		s.WaitIngest()

		assert.Len(t, s.PersonalFiles(alice), 2)
		result := &import_collection.ImportResult{OverQuota: 7}
		result.Provider, result.Total = "zvuk", 9
		result.Progress.Done, result.Progress.Failed = 2, 7
		summary := s.Catalog(alice).ImportSummary(result, adminContact)
		assert.Equal(t, []string{summary}, s.SentMessagesContaining(alice, summary))
	})
}

func TestSyncQuota(t *testing.T) {
	t.Parallel()

	t.Run("raised Quota lets the next Sync bring what did not fit", func(t *testing.T) {
		s := harness.New(t, harness.WithDefaultQuota(twoZvukTracks(t)), harness.WithSyncInterval(syncInterval))
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.ImportZvuk(alice)
		s.WaitIngest()

		s.Restart(harness.WithDefaultQuota(library.Unlimited))

		require.Eventually(t, func() bool {
			return len(s.PersonalFiles(alice)) == 9
		}, 10*time.Second, 20*time.Millisecond, "Sync never brought the tracks over the Quota")
	})

	t.Run("while the Personal Library is full Sync fetches nothing that did not fit", func(t *testing.T) {
		s := harness.New(t, harness.WithDefaultQuota(twoZvukTracks(t)), harness.WithSyncInterval(syncInterval))
		s.ConnectZvuk(alice, harness.ZvukToken)
		collection := s.AddZvukCollection(harness.ZvukToken)
		s.ImportZvuk(alice)
		s.WaitIngest()
		s.Restart(harness.WithDefaultQuota(kb))

		untilSynced(t, s, 2)
		s.WaitIngest()

		assert.Equal(t, 1, s.Zvuk.DownloadsOf(collection.Tracks[8]))
		assert.Len(t, s.PersonalFiles(alice), 2)
	})
}
