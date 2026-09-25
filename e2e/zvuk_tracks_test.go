package e2e

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.senan.xyz/taglib"
)

func TestZvukTracks(t *testing.T) {
	t.Parallel()

	t.Run("track with FLAC lands as FLAC with Zvuk's metadata", func(t *testing.T) {
		s := newScenario(t)
		s.connectZvuk(alice, zvukToken)
		s.zvuk.addRelease(zvukRelease{ID: "500", Title: "Zvuk Album", Artists: []string{"Zvuk Band"}, Date: "2021-05-14T00:00:00"})
		s.zvuk.addTrack(zvukTrack{
			ID: "101", Title: "Zvuk Song", Artists: []string{"Zvuk Band", "Guest"}, Release: "500", Position: 3,
			Seconds: 2, HasFlac: true, audio: newZvukAudio(t, 2),
		})
		s.likeOnZvuk("101")

		s.importZvuk(alice)
		s.waitIngest()

		assert.Equal(t, []string{"Zvuk Band/Zvuk Album (2021)/03 - Zvuk Song.flac"}, s.personalFiles(alice))
		path := s.personalPath(alice, "Zvuk Band/Zvuk Album (2021)/03 - Zvuk Song.flac")
		codec, _ := audioCodec(t, path)
		assert.Equal(t, "flac", codec)
		tags := readTags(t, path)
		assert.Equal(t, "Zvuk Band, Guest", tags[taglib.Artist])
		assert.Equal(t, "Zvuk Band", tags[taglib.AlbumArtist])
		assert.Equal(t, "Zvuk Album", tags[taglib.Album])
		assert.Equal(t, "2021", tags[taglib.Date])
		assert.Equal(t, "3", tags[taglib.TrackNumber])
		cover, err := taglib.ReadImage(path)
		require.NoError(t, err)
		assert.Equal(t, coverPNG, cover)
	})

	t.Run("track without FLAC lands as MP3 320", func(t *testing.T) {
		s := newScenario(t)
		s.connectZvuk(alice, zvukToken)
		s.addZvukSong("102", "Lossy Song", false)
		s.likeOnZvuk("102")

		s.importZvuk(alice)
		s.waitIngest()

		files := s.personalFiles(alice)
		require.Equal(t, []string{"Zvuk Band/Zvuk Album (2021)/01 - Lossy Song.mp3"}, files)
		assert.InDelta(t, 320, bitrateKbps(t, s.personalPath(alice, files[0])), 16)
	})

	t.Run("FLAC from Zvuk replaces the MP3 sent to Telegram", func(t *testing.T) {
		s := newScenario(t)
		s.connectZvuk(alice, zvukToken)
		s.addZvukSong("103", "Better Song", true)
		s.likeOnZvuk("103")
		s.uploaded(alice, s.uploadAudioFile(betterSongMP3(t)))

		s.importZvuk(alice)
		s.waitIngest()

		assert.Equal(t, []string{"Zvuk Band/Zvuk Album/01 - Better Song.flac"}, s.personalFiles(alice))
	})

	t.Run("replaced track is still sent by its Telegram file", func(t *testing.T) {
		s := newScenario(t)
		s.connectZvuk(alice, zvukToken)
		s.addZvukSong("103", "Better Song", true)
		s.likeOnZvuk("103")
		audio := s.uploadAudioFile(betterSongMP3(t))
		upload := s.uploaded(alice, audio)
		s.importZvuk(alice)
		s.waitIngest()
		s.share(alice, upload, "🔗 Трек")
		query := s.inlineQuery(bob, "shared")

		s.send(query)

		assert.Equal(t, []string{audio.FileID}, audioFileIDs(s.botAPI.inlineAnswerTo(t, query)))
	})

	t.Run("playlist brings every available track of every page", func(t *testing.T) {
		s := newScenario(t)
		s.connectZvuk(alice, zvukToken)
		s.zvuk.addRelease(zvukRelease{ID: "700", Title: "Mix", Artists: []string{"Various"}, Date: "2019"})
		var ids []string
		for n := 1; n <= 150; n++ {
			id := fmt.Sprint(7000 + n)
			s.zvuk.addTrack(zvukTrack{
				ID: id, Title: fmt.Sprintf("Track %d", n), Artists: []string{"Various"}, Release: "700", Position: n,
				Seconds: 2, audio: s.zvukAudio(),
			})
			ids = append(ids, id)
		}
		s.zvuk.addPlaylist(zvukPlaylist{ID: "800", Title: "Long Playlist", Tracks: ids})
		s.zvuk.makeUnavailable(ids[10])
		s.zvuk.update(zvukToken, func(a *zvukAccount) { a.playlists = []string{"800"} })

		s.importZvuk(alice)
		s.waitIngest()

		assert.Len(t, s.personalFiles(alice), 149)
		assert.Contains(t, s.progressMessage(t).Text, "149 из 149 треков в библиотеке")
	})

	t.Run("unavailable Zvuk does not stop Telegram uploads", func(t *testing.T) {
		s := newScenario(t)
		s.connectZvuk(alice, zvukToken)
		s.addZvukCollection(zvukToken)
		s.zvuk.holdStreams(0)
		s.importZvuk(alice)
		s.zvuk.setDown(true)
		upload := s.audioMessage(alice, s.uploadAudio("track.mp3"))

		s.send(upload)

		require.Eventually(t, func() bool {
			return len(s.botAPI.reactionsOn(t, upload.Message.ID)) == 2
		}, 10*time.Second, 20*time.Millisecond)
		assert.Equal(t, []string{"👀", "👍"}, s.botAPI.reactionsOn(t, upload.Message.ID))
		assert.Contains(t, s.personalFiles(alice), fixtureTrackPath)
	})
}

func (s *scenario) likeOnZvuk(ids ...string) {
	s.t.Helper()
	s.zvuk.update(zvukToken, func(a *zvukAccount) { a.liked = ids })
}

// betterSongMP3 is the MP3 of "Better Song" a user might have sent to
// Telegram before finding the track in Zvuk.
func betterSongMP3(t *testing.T) string {
	return makeAudio(t, "better.mp3", audioSpec{Seconds: 2, Tags: map[string]string{
		"artist": "Zvuk Band", "album": "Zvuk Album", "title": "Better Song", "track": "1",
	}})
}

// addZvukSong's tracks all belong to "Zvuk Band — Zvuk Album (2021)".
func (s *scenario) addZvukSong(id, title string, hasFlac bool) {
	s.t.Helper()

	s.zvuk.addRelease(zvukRelease{ID: "500", Title: "Zvuk Album", Artists: []string{"Zvuk Band"}, Date: "2021-01-01"})
	s.zvuk.addTrack(zvukTrack{
		ID: id, Title: title, Artists: []string{"Zvuk Band"}, Release: "500", Position: 1,
		Seconds: 2, HasFlac: hasFlac, audio: s.zvukAudio(),
	})
}

// zvukAudio shares one set of files among the scenario's tracks: Zvuk's
// metadata tells them apart.
func (s *scenario) zvukAudio() zvukAudio {
	if s.sharedZvukAudio == (zvukAudio{}) {
		s.sharedZvukAudio = newZvukAudio(s.t, 2)
	}
	return s.sharedZvukAudio
}

// addZvukAlbum adds "Zvuk Band — title (2020)" with n MP3-only tracks
// numbered from 1, their ids made of the album's.
func (s *scenario) addZvukAlbum(id, title string, n int) []string {
	s.t.Helper()

	s.zvuk.addRelease(zvukRelease{ID: id, Title: title, Artists: []string{"Zvuk Band"}, Date: "2020-02-02"})
	var ids []string
	for i := 1; i <= n; i++ {
		trackID := fmt.Sprintf("%s%02d", id, i)
		s.zvuk.addTrack(zvukTrack{
			ID: trackID, Title: fmt.Sprintf("Song %d", i), Artists: []string{"Zvuk Band"}, Release: id, Position: i,
			Seconds: 2, audio: s.zvukAudio(),
		})
		ids = append(ids, trackID)
	}
	return ids
}
