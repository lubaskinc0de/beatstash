package e2e

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/beatstash/e2e/harness"
	"github.com/lubaskinc0de/beatstash/e2e/harness/audiofile"
	"github.com/lubaskinc0de/beatstash/e2e/harness/navidrome"
	"github.com/lubaskinc0de/beatstash/e2e/harness/telegram"
	appnd "github.com/lubaskinc0de/beatstash/internal/application/common/navidrome"
	"github.com/lubaskinc0de/beatstash/internal/application/show_playing"
	"github.com/lubaskinc0de/beatstash/internal/domain/library"
)

func TestNowPlayingAudio(t *testing.T) {
	t.Parallel()

	t.Run("chosen np turns into audio and the next np is audio", func(t *testing.T) {
		s := harness.New(t, harness.WithStorageChat(storageChat), harness.WithoutStorageFill())
		playingUnpostedZvukSong(t, s, alice, "Pending Song")
		result := s.NowPlaying(alice)

		chosen := s.Choose(alice, result)
		next := s.NowPlaying(alice)

		assert.Equal(t, "article", result.Type)
		assert.Contains(t, result.Content.Text, "⏳")
		edits := s.Telegram.InlineEdits(t, chosen.ChosenInlineResult.InlineMessageID)
		require.Len(t, edits, 1)
		assert.Equal(t, "editMessageMedia", edits[0].Method)
		assert.Equal(t, "audio", edits[0].Media.Type)
		assert.Equal(t, s.Telegram.UploadedFileID(0), edits[0].Media.Media)
		assert.Contains(t, edits[0].Media.Caption, "Pending Song")
		assert.NotContains(t, edits[0].Media.Caption, "⏳")
		assert.Empty(t, edits[0].Buttons)
		assert.Equal(t, "audio", next.Type)
		assert.Equal(t, s.Telegram.UploadedFileID(0), next.AudioFileID)
	})

	t.Run("file over the upload limit without Listen Links leaves text with a note", func(t *testing.T) {
		s := harness.New(t,
			harness.WithStorageChat(storageChat), harness.WithoutStorageFill(), harness.WithMaxUpload(1024), harness.WithPublicURL(""))
		playingUnpostedZvukSong(t, s, alice, "Huge Song")

		chosen := s.Choose(alice, s.NowPlaying(alice))

		edits := s.Telegram.InlineEdits(t, chosen.ChosenInlineResult.InlineMessageID)
		require.Len(t, edits, 1)
		assert.Equal(t, "editMessageText", edits[0].Method)
		assert.Contains(t, edits[0].Text, "Huge Song")
		assert.Contains(t, edits[0].Text, s.Catalog(alice).NotSentNote(true))
		assert.NotContains(t, edits[0].Text, "⏳")
	})

	t.Run("chosen recent track turns into audio", func(t *testing.T) {
		s := harness.New(t, harness.WithStorageChat(storageChat), harness.WithoutStorageFill())
		account, song := unpostedZvukSong(t, s, alice, "Played Song")
		s.Navidrome.Play(t, account, song.ID)
		query := s.InlineQuery(alice, "recent")
		s.Send(query)
		results := s.Telegram.InlineAnswerTo(t, query).Results
		require.Len(t, results, 2)
		result := results[1]

		chosen := s.Choose(alice, result)

		assert.Contains(t, result.Content.Text, "⏳")
		assert.Equal(t, []string{"⏳"}, telegram.ButtonTexts(result.Buttons()))
		edits := s.Telegram.InlineEdits(t, chosen.ChosenInlineResult.InlineMessageID)
		require.Len(t, edits, 1)
		assert.Equal(t, "editMessageMedia", edits[0].Method)
		assert.Equal(t, s.Telegram.UploadedFileID(0), edits[0].Media.Media)
		assert.Empty(t, edits[0].Buttons)
	})

	t.Run("chosen np FLAC becomes its audio under the np text", func(t *testing.T) {
		s := harness.New(t)
		account := s.LinkNewAccount(alice)
		audio := s.UploadAudio("track.flac")
		s.Uploaded(alice, audio)
		s.Navidrome.StartPlaying(t, account, s.Navidrome.IndexedTrack(t, account, s.Library, audiofile.FixtureTitle).ID)
		result := s.NowPlaying(alice)

		chosen := s.Choose(alice, result)

		assert.Contains(t, result.Content.Text, "⏳")
		edits := s.Telegram.InlineEdits(t, chosen.ChosenInlineResult.InlineMessageID)
		require.Len(t, edits, 1)
		assert.Equal(t, "editMessageMedia", edits[0].Method)
		assert.Equal(t, "audio", edits[0].Media.Type)
		assert.Equal(t, audio.FileID, edits[0].Media.Media)
		assert.Equal(t, nowPlayingText(t, s, alice, result, library.Metadata{
			Artist: "Fixture Artist", Title: audiofile.FixtureTitle, Album: "Fixture Album",
		}), edits[0].Media.Caption)
	})

	t.Run("recent with FLAC and MP3 answers in full", func(t *testing.T) {
		s := harness.New(t)
		account := s.LinkNewAccount(alice)
		mp3 := s.UploadAudioFile(mixedSong(t, "mp3.mp3", "MP3 Song"))
		s.Uploaded(alice, mp3)
		flac := s.UploadAudioFile(mixedSong(t, "flac.flac", "FLAC Song"))
		s.Uploaded(alice, flac)
		for _, title := range []string{"MP3 Song", "FLAC Song"} {
			s.Navidrome.Play(t, account, s.Navidrome.IndexedTrack(t, account, s.Library, title).ID)
		}
		answer := s.Search(alice, "recent", "")
		require.Len(t, answer.Results, 3)

		chosen := s.Choose(alice, answer.Results[1])

		assert.Equal(t, []string{mp3.FileID}, telegram.AudioFileIDs(answer))
		assert.Contains(t, answer.Results[1].Content.Text, "⏳")
		edits := s.Telegram.InlineEdits(t, chosen.ChosenInlineResult.InlineMessageID)
		require.Len(t, edits, 1)
		assert.Equal(t, "editMessageMedia", edits[0].Method)
		assert.Equal(t, flac.FileID, edits[0].Media.Media)
	})

	t.Run("simultaneous choices upload the file once", func(t *testing.T) {
		s := harness.New(t, harness.WithStorageChat(storageChat), harness.WithoutStorageFill())
		playingUnpostedZvukSong(t, s, alice, "Pending Song")
		result := s.NowPlaying(alice)
		choices := []*models.Update{s.ChosenResult(alice, result), s.ChosenResult(alice, result), s.ChosenResult(alice, result)}

		var wg sync.WaitGroup
		for _, choice := range choices {
			wg.Go(func() { s.Send(choice) })
		}
		wg.Wait()

		assert.Empty(t, s.Telegram.UploadedFileID(1))
		for _, choice := range choices {
			edits := s.Telegram.InlineEdits(t, choice.ChosenInlineResult.InlineMessageID)
			require.Len(t, edits, 1)
			assert.Equal(t, s.Telegram.UploadedFileID(0), edits[0].Media.Media)
		}
	})
}

// nowPlayingText is the np text of what the result showed, at the
// position it showed, for a playing Track the bot holds with the metadata.
func nowPlayingText(t *testing.T, s *harness.Scenario, user harness.User, result telegram.InlineResult, m library.Metadata) string {
	t.Helper()

	artist, title, album := m.Artist, m.Title, m.Album

	var posMin, posSec, durMin, durSec int
	_, err := fmt.Sscanf(strings.TrimPrefix(result.Description, album+" · "), "%d:%d / %d:%d", &posMin, &posSec, &durMin, &durSec)
	require.NoError(t, err, "np description %q", result.Description)
	return s.Catalog(user).NowPlaying(&show_playing.NowPlaying{PlayingTrack: appnd.PlayingTrack{
		Track:      appnd.Track{Artist: artist, Title: title, Album: album, Duration: durMin*60 + durSec},
		PositionMs: (posMin*60 + posSec) * 1000,
		State:      "playing",
	}, Track: &library.Track{Metadata: m}})
}

func mixedSong(t *testing.T, name, title string) string {
	t.Helper()

	return audiofile.Generate(t, name, audiofile.Spec{Tags: map[string]string{
		"artist": "Mixed Band", "album": "Mixed Album", "title": title, "track": "1",
	}})
}

func playingUnpostedZvukSong(t *testing.T, s *harness.Scenario, user harness.User, title string) {
	t.Helper()

	account, song := unpostedZvukSong(t, s, user, title)
	s.Navidrome.StartPlaying(t, account, song.ID)
}

// unpostedZvukSong has the user import a Zvuk track; with the storage chat
// filled only on demand it has no Telegram file yet.
func unpostedZvukSong(t *testing.T, s *harness.Scenario, user harness.User, title string) (navidrome.Account, navidrome.Track) {
	t.Helper()

	account := s.LinkNewAccount(user)
	s.ConnectZvuk(user, harness.ZvukToken)
	s.AddZvukSong("111", title, false)
	s.LikeOnZvuk("111")
	s.ImportZvuk(user)
	s.WaitIngest()
	return account, s.Navidrome.IndexedTrack(t, account, s.PersonalPath(user, ""), title)
}
