package e2e

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.senan.xyz/taglib"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/audiofile"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/telegram"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/zvuk"
)

func TestZvukTracks(t *testing.T) {
	t.Parallel()

	t.Run("track with FLAC lands as FLAC with Zvuk's metadata", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.Zvuk.AddRelease(zvuk.Release{ID: "500", Title: "Zvuk Album", Artists: []string{"Zvuk Band"}, Date: "2021-05-14T00:00:00"})
		s.Zvuk.AddTrack(zvuk.Track{
			ID: "101", Title: "Zvuk Song", Artists: []string{"Zvuk Band", "Guest"}, Release: "500", Position: 3,
			Seconds: 2, HasFlac: true, Audio: zvuk.NewAudio(t, 2),
		})
		s.LikeOnZvuk("101")

		s.ImportZvuk(alice)
		s.WaitIngest()

		assert.Equal(t, []string{"Zvuk Band/Zvuk Album (2021)/03 - Zvuk Song.flac"}, s.PersonalFiles(alice))
		path := s.PersonalPath(alice, "Zvuk Band/Zvuk Album (2021)/03 - Zvuk Song.flac")
		codec, _ := audiofile.Codec(t, path)
		assert.Equal(t, "flac", codec)
		tags := audiofile.Tags(t, path)
		assert.Equal(t, "Zvuk Band, Guest", tags[taglib.Artist])
		assert.Equal(t, "Zvuk Band", tags[taglib.AlbumArtist])
		assert.Equal(t, "Zvuk Album", tags[taglib.Album])
		assert.Equal(t, "2021", tags[taglib.Date])
		assert.Equal(t, "3", tags[taglib.TrackNumber])
		cover, err := taglib.ReadImage(path)
		require.NoError(t, err)
		assert.Equal(t, zvuk.Cover, cover)
	})

	t.Run("track without FLAC lands as MP3 320", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukSong("102", "Lossy Song", false)
		s.LikeOnZvuk("102")

		s.ImportZvuk(alice)
		s.WaitIngest()

		files := s.PersonalFiles(alice)
		require.Equal(t, []string{"Zvuk Band/Zvuk Album (2021)/01 - Lossy Song.mp3"}, files)
		assert.InDelta(t, 320, audiofile.BitrateKbps(t, s.PersonalPath(alice, files[0])), 16)
	})

	t.Run("FLAC from Zvuk replaces the MP3 sent to Telegram", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukSong("103", "Better Song", true)
		s.LikeOnZvuk("103")
		s.Uploaded(alice, s.UploadAudioFile(betterSongMP3(t)))

		s.ImportZvuk(alice)
		s.WaitIngest()

		assert.Equal(t, []string{"Zvuk Band/Zvuk Album/01 - Better Song.flac"}, s.PersonalFiles(alice))
	})

	t.Run("replaced track is still sent by its Telegram file", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukSong("103", "Better Song", true)
		s.LikeOnZvuk("103")
		audio := s.UploadAudioFile(betterSongMP3(t))
		upload := s.Uploaded(alice, audio)
		s.ImportZvuk(alice)
		s.WaitIngest()
		s.Share(alice, upload, "🔗 Трек")
		query := s.InlineQuery(bob, "shared")

		s.Send(query)

		assert.Equal(t, []string{audio.FileID}, telegram.AudioFileIDs(s.Telegram.InlineAnswerTo(t, query)))
	})

	t.Run("playlist brings every available track of every page", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.Zvuk.AddRelease(zvuk.Release{ID: "700", Title: "Mix", Artists: []string{"Various"}, Date: "2019"})
		var ids []string
		for n := 1; n <= 150; n++ {
			id := fmt.Sprint(7000 + n)
			s.Zvuk.AddTrack(zvuk.Track{
				ID: id, Title: fmt.Sprintf("Track %d", n), Artists: []string{"Various"}, Release: "700", Position: n,
				Seconds: 2, Audio: s.ZvukAudio(),
			})
			ids = append(ids, id)
		}
		s.Zvuk.AddPlaylist(zvuk.Playlist{ID: "800", Title: "Long Playlist", Tracks: ids})
		s.Zvuk.MakeUnavailable(ids[10])
		s.Zvuk.Update(harness.ZvukToken, func(a *zvuk.Account) { a.Playlists = []string{"800"} })

		s.ImportZvuk(alice)
		s.WaitIngest()

		assert.Len(t, s.PersonalFiles(alice), 149)
		assert.Contains(t, s.SentMessagesContaining("Импорт из Звука")[0], "149 из 149 в библиотеке")
	})

	t.Run("unavailable Zvuk does not stop Telegram uploads", func(t *testing.T) {
		s := harness.New(t)
		s.ConnectZvuk(alice, harness.ZvukToken)
		s.AddZvukCollection(harness.ZvukToken)
		s.Zvuk.HoldStreams(0)
		s.ImportZvuk(alice)
		s.Zvuk.SetDown(true)
		upload := s.AudioMessage(alice, s.UploadAudio("track.mp3"))

		s.Send(upload)

		require.Eventually(t, func() bool {
			return len(s.Telegram.ReactionsOn(t, upload.Message.ID)) == 2
		}, 10*time.Second, 20*time.Millisecond)
		assert.Equal(t, []string{"👀", "👍"}, s.Telegram.ReactionsOn(t, upload.Message.ID))
		assert.Contains(t, s.PersonalFiles(alice), audiofile.FixtureTrackPath)
	})
}

// betterSongMP3 is the MP3 of "Better Song" a user might have sent to
// Telegram before finding the track in Zvuk.
func betterSongMP3(t *testing.T) string {
	return audiofile.Generate(t, "better.mp3", audiofile.Spec{Seconds: 2, Tags: map[string]string{
		"artist": "Zvuk Band", "album": "Zvuk Album", "title": "Better Song", "track": "1",
	}})
}
