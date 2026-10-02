package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/audiofile"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/navidrome"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/telegram"
)

func TestSearchMusic(t *testing.T) {
	t.Parallel()

	t.Run("artist finds the user's track as audio", func(t *testing.T) {
		s := harness.New(t)
		audio := s.UploadAudio("track.mp3")
		s.Uploaded(alice, audio)

		results := s.Search(alice, "fixture artist", "").Results

		require.Len(t, results, 1)
		assert.Equal(t, "audio", results[0].Type)
		assert.Equal(t, audio.FileID, results[0].AudioFileID)
		assert.Contains(t, results[0].Caption, audiofile.FixtureTitle)
	})

	t.Run("words from different fields find the track", func(t *testing.T) {
		s := harness.New(t)
		s.Uploaded(alice, s.UploadAudioFile(queenSong(t)))
		s.Uploaded(alice, s.UploadAudio("track.mp3"))

		results := s.Search(alice, "QUEEN rhapsody", "").Results

		require.Len(t, results, 1)
		assert.Contains(t, results[0].Caption, "Bohemian Rhapsody")
	})

	t.Run("е finds ё", func(t *testing.T) {
		s := harness.New(t)
		s.Uploaded(alice, s.UploadAudioFile(audiofile.Generate(t, "yolka.mp3", audiofile.Spec{Tags: map[string]string{
			"artist": "Ёлка", "album": "Альбом", "title": "Прованс", "track": "1",
		}})))

		results := s.Search(alice, "елка", "").Results

		require.Len(t, results, 1)
		assert.Contains(t, results[0].Caption, "Прованс")
	})

	t.Run("track shared by another user is found", func(t *testing.T) {
		s := harness.New(t)
		audio := s.UploadAudio("track.mp3")
		s.Share(alice, s.Uploaded(alice, audio), s.Catalog(alice).ShareTrack())

		results := s.Search(bob, audiofile.FixtureTitle, "").Results

		require.Len(t, results, 1)
		assert.Equal(t, audio.FileID, results[0].AudioFileID)
	})

	t.Run("track both kept and shared is found once", func(t *testing.T) {
		s := harness.New(t)
		s.Share(alice, s.Uploaded(alice, s.UploadAudio("track.mp3")), s.Catalog(alice).ShareTrack())

		results := s.Search(alice, audiofile.FixtureTitle, "").Results

		assert.Len(t, results, 1)
	})

	t.Run("another user's Personal Library is not searched", func(t *testing.T) {
		s := harness.New(t)
		s.Uploaded(bob, s.UploadAudio("track.mp3"))

		results := s.Search(alice, audiofile.FixtureTitle, "").Results

		require.Len(t, results, 1)
		assert.Equal(t, s.Catalog(alice).NothingFound().Title, results[0].Title)
	})

	t.Run("Attached Library the user does not see is not searched", func(t *testing.T) {
		var account navidrome.Account
		s := harness.NewOwnNavidrome(t, func(s *harness.Scenario) {
			account = s.Navidrome.CreateAccount(t, "alice")
			other := s.NewNavidromeLibrary("other", audiofile.Fixture("track.mp3"))
			s.Navidrome.UntilSongs(t, other.ID, 1)
		})
		s.Link(alice, account)

		results := s.Search(alice, audiofile.FixtureTitle, "").Results

		require.Len(t, results, 1)
		assert.Equal(t, s.Catalog(alice).NothingFound().Title, results[0].Title)
	})

	t.Run("next page comes by the offset", func(t *testing.T) {
		s := harness.New(t)
		uploadMany(t, s, alice, 21)
		first := s.Search(alice, "many", "")

		second := s.Search(alice, "many", first.NextOffset)

		assert.Len(t, first.Results, 20)
		assert.Len(t, second.Results, 1)
		assert.Empty(t, second.NextOffset)
		assert.NotContains(t, telegram.AudioFileIDs(first), second.Results[0].AudioFileID)
	})

	t.Run("track without a Telegram file is sent as audio once chosen", func(t *testing.T) {
		s := harness.New(t, harness.WithStorageChat(storageChat), harness.WithoutStorageFill())
		importZvukSong(s, alice, "Pending Song")
		results := s.Search(alice, "pending song", "").Results
		require.Len(t, results, 1)

		chosen := s.Choose(alice, results[0])

		assert.Contains(t, results[0].Content.Text, "⏳")
		edits := s.Telegram.InlineEdits(t, chosen.ChosenInlineResult.InlineMessageID)
		require.Len(t, edits, 1)
		assert.Equal(t, "editMessageMedia", edits[0].Method)
		assert.Equal(t, s.Telegram.UploadedFileID(0), edits[0].Media.Media)
		assert.Empty(t, edits[0].Buttons)
	})
}

func TestSearchMusicTypos(t *testing.T) {
	t.Parallel()

	t.Run("typos still find the track", func(t *testing.T) {
		s := harness.New(t)
		s.Uploaded(alice, s.UploadAudioFile(queenSong(t)))

		results := s.Search(alice, "quen bohemain", "").Results

		require.Len(t, results, 1)
		assert.Contains(t, results[0].Caption, "Bohemian Rhapsody")
	})

	t.Run("words that match bring no similar tracks", func(t *testing.T) {
		s := harness.New(t)
		s.Uploaded(alice, s.UploadAudioFile(queenSong(t)))
		s.Uploaded(alice, s.UploadAudioFile(audiofile.Generate(t, "queem.mp3", audiofile.Spec{Tags: map[string]string{
			"artist": "Queem", "album": "Other", "title": "Other Song", "track": "1",
		}})))

		results := s.Search(alice, "queen", "").Results

		require.Len(t, results, 1)
		assert.Contains(t, results[0].Caption, "Bohemian Rhapsody")
	})

	t.Run("nonsense finds nothing", func(t *testing.T) {
		s := harness.New(t)
		s.Uploaded(alice, s.UploadAudioFile(queenSong(t)))

		results := s.Search(alice, "zxqwv", "").Results

		require.Len(t, results, 1)
		assert.Equal(t, s.Catalog(alice).NothingFound().Title, results[0].Title)
	})
}

func queenSong(t *testing.T) string {
	t.Helper()
	return audiofile.Generate(t, "queen.mp3", audiofile.Spec{Tags: map[string]string{
		"artist": "Queen", "album": "A Night at the Opera", "title": "Bohemian Rhapsody", "track": "11",
	}})
}

func uploadMany(t *testing.T, s *harness.Scenario, user harness.User, n int) {
	t.Helper()

	for i := 1; i <= n; i++ {
		s.Send(s.AudioMessage(user, s.UploadAudioFile(numberedTrack(t, i))))
	}
	s.WaitIngest()
}
