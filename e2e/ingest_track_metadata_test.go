package e2e

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/audiofile"
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
			s := harness.New(t)
			audio := s.UploadAudioFile(audiofile.Generate(t, "in.mp3", audiofile.Spec{Tags: tc.tags}))

			s.Uploaded(alice, audio)

			assert.Equal(t, []string{tc.path}, s.PersonalFiles(alice))
		})
	}

	t.Run("different tracks with one path both land", func(t *testing.T) {
		s := harness.New(t)
		tags := map[string]string{"artist": "Artist", "album": "Album", "track": "1", "title": "Song"}
		short := s.UploadAudioFile(audiofile.Generate(t, "short.mp3", audiofile.Spec{Seconds: 1.5, Tags: tags}))
		long := s.UploadAudioFile(audiofile.Generate(t, "long.mp3", audiofile.Spec{Seconds: 8, Tags: tags}))
		s.Uploaded(alice, short)

		s.Uploaded(alice, long)

		assert.Equal(t, []string{"Artist/Album/01 - Song (2).mp3", "Artist/Album/01 - Song.mp3"}, s.PersonalFiles(alice))
	})

	t.Run("album sent track by track shares one folder", func(t *testing.T) {
		s := harness.New(t)
		for i, title := range []string{"One", "Two", "Three"} {
			audio := s.UploadAudioFile(audiofile.Generate(t, title+".mp3", audiofile.Spec{Tags: map[string]string{
				"artist": "Band", "album": "Record", "date": "2001", "track": string(rune('1' + i)), "title": title,
			}}))
			s.Send(s.AudioMessage(alice, audio))
		}

		s.WaitIngest()

		assert.Equal(t, []string{
			"Band/Record (2001)/01 - One.mp3",
			"Band/Record (2001)/02 - Two.mp3",
			"Band/Record (2001)/03 - Three.mp3",
		}, s.PersonalFiles(alice))
	})
}

func TestMetadata(t *testing.T) {
	t.Parallel()

	t.Run("untagged MP3 is laid out by Telegram data", func(t *testing.T) {
		s := harness.New(t)
		audio := s.UploadAudioFile(audiofile.Generate(t, "untagged.mp3", audiofile.Spec{}))
		audio.Performer = "Telegram Artist"
		audio.Title = "Telegram Song"

		s.Uploaded(alice, audio)

		assert.Equal(t, []string{"👀", "👍"}, s.Telegram.Reactions(t))
		assert.Equal(t, []string{"Telegram Artist/Singles/Telegram Song.mp3"}, s.PersonalFiles(alice))
		tags := audiofile.Tags(t, s.PersonalPath(alice, "Telegram Artist/Singles/Telegram Song.mp3"))
		assert.Equal(t, "Telegram Artist", tags["ARTIST"])
		assert.Equal(t, "Telegram Song", tags["TITLE"])
	})

	t.Run("untagged file is laid out by its name", func(t *testing.T) {
		s := harness.New(t)
		audio := s.UploadAudioFile(audiofile.Generate(t, "Name Artist - Name Song.mp3", audiofile.Spec{}))

		s.Uploaded(alice, audio)

		assert.Equal(t, []string{"Name Artist/Singles/Name Song.mp3"}, s.PersonalFiles(alice))
		tags := audiofile.Tags(t, s.PersonalPath(alice, "Name Artist/Singles/Name Song.mp3"))
		assert.Equal(t, "Name Artist", tags["ARTIST"])
		assert.Equal(t, "Name Song", tags["TITLE"])
	})

	t.Run("unrecognized track goes to Inbox", func(t *testing.T) {
		s := harness.New(t)
		msg := s.AudioMessage(alice, s.UploadAudioFile(audiofile.Generate(t, "audio_123.mp3", audiofile.Spec{})))

		s.Send(msg)
		s.WaitIngest()

		assert.Equal(t, []string{"👀", "👍"}, s.Telegram.Reactions(t))
		assert.Equal(t, []string{"Inbox/audio_123.mp3"}, s.PersonalFiles(alice))
		replies := s.Telegram.Replies(t)
		require.Len(t, replies, 1)
		assert.Equal(t, msg.Message.ID, replies[0].ReplyTo)
		assert.Contains(t, replies[0].Text, "«Входящие»")
	})

	t.Run("file tags beat Telegram data", func(t *testing.T) {
		s := harness.New(t)
		audio := s.UploadAudio("track.mp3")
		audio.Performer = "Telegram Artist"
		audio.Title = "Telegram Song"

		s.Uploaded(alice, audio)

		assert.Equal(t, []string{audiofile.FixtureTrackPath}, s.PersonalFiles(alice))
	})

	t.Run("Telegram data fills fields tags lack", func(t *testing.T) {
		s := harness.New(t)
		audio := s.UploadAudioFile(audiofile.Generate(t, "partial.mp3", audiofile.Spec{
			Tags: map[string]string{"title": "Tagged Title", "album": "Tagged Album"},
		}))
		audio.Performer = "Telegram Artist"
		audio.Title = "Telegram Song"

		s.Uploaded(alice, audio)

		assert.Equal(t, []string{"Telegram Artist/Tagged Album/Tagged Title.mp3"}, s.PersonalFiles(alice))
	})
}
