package e2e

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLayout(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		tags map[string]string
		path string
	}{
		{
			name: "album with year and number",
			tags: map[string]string{"artist": "Artist", "album": "Album", "date": "1999", "track": "7", "title": "Song"},
			path: "Artist/Album (1999)/07 - Song.mp3",
		},
		{
			name: "album without year",
			tags: map[string]string{"artist": "Artist", "album": "Album", "track": "7", "title": "Song"},
			path: "Artist/Album/07 - Song.mp3",
		},
		{
			name: "album without number",
			tags: map[string]string{"artist": "Artist", "album": "Album", "date": "1999", "title": "Song"},
			path: "Artist/Album (1999)/Song.mp3",
		},
		{
			name: "number of two digits and more",
			tags: map[string]string{"artist": "Artist", "album": "Album", "track": "112/120", "title": "Song"},
			path: "Artist/Album/112 - Song.mp3",
		},
		{
			name: "album artist groups the album",
			tags: map[string]string{"artist": "Guest", "album_artist": "Various Artists", "album": "Hits", "track": "2", "title": "Song"},
			path: "Various Artists/Hits/02 - Song.mp3",
		},
		{
			name: "single",
			tags: map[string]string{"artist": "Artist", "date": "1999", "title": "Song"},
			path: "Artist/Singles/Song.mp3",
		},
		{
			name: "forbidden characters",
			tags: map[string]string{"artist": "AC/DC", "album": `Who: Made? Who*`, "title": `Say "Hi" <now> | a\b`},
			path: `AC_DC/Who_ Made_ Who_/Say _Hi_ _now_ _ a_b.mp3`,
		},
		{
			name: "trailing and leading dots",
			tags: map[string]string{"artist": "Mr. Ed.", "album": ".Vol. 1...", "title": "End. "},
			path: "Mr. Ed/Vol. 1/End.mp3",
		},
		{
			name: "overlong name",
			tags: map[string]string{"artist": "Artist", "title": strings.Repeat("a", 300)},
			path: "Artist/Singles/" + strings.Repeat("a", 200) + ".mp3",
		},
		{
			name: "overlong name cut between characters",
			tags: map[string]string{"artist": "Artist", "title": strings.Repeat("я", 150)},
			path: "Artist/Singles/" + strings.Repeat("я", 100) + ".mp3",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newScenario(t)
			audio := s.uploadAudioFile(makeAudio(t, "in.mp3", audioSpec{Tags: tc.tags}))

			s.send(s.audioMessage(alice, audio))
			s.waitIngest()

			assert.Equal(t, []string{tc.path}, s.personalFiles(alice))
		})
	}

	t.Run("different tracks with one path both land", func(t *testing.T) {
		s := newScenario(t)
		tags := map[string]string{"artist": "Artist", "album": "Album", "track": "1", "title": "Song"}
		short := s.uploadAudioFile(makeAudio(t, "short.mp3", audioSpec{Seconds: 1.5, Tags: tags}))
		long := s.uploadAudioFile(makeAudio(t, "long.mp3", audioSpec{Seconds: 8, Tags: tags}))
		s.send(s.audioMessage(alice, short))
		s.waitIngest()

		s.send(s.audioMessage(alice, long))
		s.waitIngest()

		assert.Equal(t, []string{"Artist/Album/01 - Song (2).mp3", "Artist/Album/01 - Song.mp3"}, s.personalFiles(alice))
	})

	t.Run("album sent track by track shares one folder", func(t *testing.T) {
		s := newScenario(t)
		for i, title := range []string{"One", "Two", "Three"} {
			audio := s.uploadAudioFile(makeAudio(t, title+".mp3", audioSpec{Tags: map[string]string{
				"artist": "Band", "album": "Record", "date": "2001", "track": string(rune('1' + i)), "title": title,
			}}))
			s.send(s.audioMessage(alice, audio))
		}

		s.waitIngest()

		assert.Equal(t, []string{
			"Band/Record (2001)/01 - One.mp3",
			"Band/Record (2001)/02 - Two.mp3",
			"Band/Record (2001)/03 - Three.mp3",
		}, s.personalFiles(alice))
	})
}
