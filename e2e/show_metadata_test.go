package e2e

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.senan.xyz/taglib"

	"github.com/lubaskinc0de/beatstash/e2e/harness"
	"github.com/lubaskinc0de/beatstash/e2e/harness/audiofile"
	"github.com/lubaskinc0de/beatstash/e2e/harness/zvuk"
)

func TestShowMetadata(t *testing.T) {
	t.Parallel()

	t.Run("track result shows album, year, genres and duration; its audio the rest", func(t *testing.T) {
		s := harness.New(t)
		s.Uploaded(alice, s.UploadAudioFile(paranoidAndroid(t)))
		results := s.Search(alice, "paranoid", "").Results
		require.Len(t, results, 1)

		chosen := s.Choose(alice, results[0])

		assert.Equal(t, "💿 OK Computer · 1997 · Alternative Rock, Art Rock · 00:02", results[0].Description)
		edits := s.Telegram.InlineEdits(t, chosen.ChosenInlineResult.InlineMessageID)
		require.Len(t, edits, 1)
		assert.Equal(t, "💿 OK Computer (1997)\n🎼 Alternative Rock, Art Rock · 🏷 Parlophone", edits[0].Media.Caption)
	})

	t.Run("track card shows metadata and sends the file under it", func(t *testing.T) {
		s := harness.New(t)
		importZvukRiffs(s)
		c := s.Catalog(alice)
		s.Open(alice, c.MusicButton(), c.MineTab(), "Zvuk Band — Riff")
		card := s.WindowText()
		send := s.Button(c.SendOwnFileButton())

		s.Press(alice, send)
		s.Press(alice, send)

		assert.Contains(t, card, "🎧 <b>Zvuk Band</b> — Riff\n💿 Riffs (2019)\n🎼 Rock, Hard Rock · 🏷 Zvuk Records\n📚")
		sent := s.Telegram.CallsTo("sendAudio")
		require.Len(t, sent, 2)
		for _, call := range sent {
			assert.Equal(t, "💿 Riffs (2019)\n🎼 Rock, Hard Rock · 🏷 Zvuk Records", call.Params["caption"])
		}
	})

	t.Run("Zvuk track without genres takes the release's, and its file keeps them", func(t *testing.T) {
		s := harness.New(t)

		importZvukRiffs(s)

		c := s.Catalog(alice)
		s.Open(alice, c.MusicButton(), c.MineTab(), "Zvuk Band — Ballad")
		assert.Contains(t, s.WindowText(), "\n🎼 Pop · 🏷 Zvuk Records\n")
		tags := audiofile.Tags(t, s.PersonalPath(alice, "Zvuk Band/Riffs (2019)/02 - Ballad.mp3"))
		assert.Equal(t, "Pop", tags[taglib.Genre])
		assert.Equal(t, "Zvuk Records", tags[taglib.Label])
	})

	t.Run("now playing shows the metadata of the playing track", func(t *testing.T) {
		s := harness.New(t, harness.WithStorageChat(storageChat), harness.WithoutStorageFill())
		account := s.LinkNewAccount(alice)
		importZvukRiffs(s)
		s.Navidrome.StartPlaying(t, account, s.Navidrome.IndexedTrack(t, account, s.PersonalPath(alice, ""), "Riff").ID)

		chosen := s.Choose(alice, s.NowPlaying(alice))

		edits := s.Telegram.InlineEdits(t, chosen.ChosenInlineResult.InlineMessageID)
		require.Len(t, edits, 1)
		assert.Contains(t, edits[0].Media.Caption, "👤 Zvuk Band\n💿 Riffs (2019)\n🎼 Rock, Hard Rock · 🏷 Zvuk Records\n\n")
	})

	t.Run("attached library track takes genres and label from Navidrome", func(t *testing.T) {
		s, account, _ := newWithOwnLibrary(t, []string{paranoidAndroid(t)})
		s.Link(alice, account)
		c := s.Catalog(alice)

		s.Open(alice, c.MusicButton(), c.MineTab(), "Radiohead — Paranoid Android")

		assert.Contains(t, s.WindowText(), "\n💿 OK Computer (1997)\n🎼 Alternative Rock, Art Rock · 🏷 Parlophone\n")
	})

	t.Run("better copy of a track fills its missing genres and label", func(t *testing.T) {
		s := harness.New(t)
		s.Uploaded(alice, s.UploadAudioFile(audiofile.Generate(t, "paranoid-plain.mp3", audiofile.Spec{Seconds: 2, Bitrate: "128k", Tags: map[string]string{
			"artist": "Radiohead", "album": "OK Computer", "title": "Paranoid Android", "track": "2", "date": "1997",
		}})))

		s.Uploaded(alice, s.UploadAudioFile(paranoidAndroid(t)))

		c := s.Catalog(alice)
		s.Open(alice, c.MusicButton(), c.MineTab(), "Radiohead — Paranoid Android")
		assert.Contains(t, s.WindowText(), "\n🎼 Alternative Rock, Art Rock · 🏷 Parlophone\n")
	})

	t.Run("album shows the most common year, genre and label of its tracks", func(t *testing.T) {
		s := harness.New(t)
		account := s.LinkNewAccount(alice)
		for n, tags := range []map[string]string{
			{"date": "2000", "genre": "Electronic", "publisher": "Parlophone"},
			{"date": "2000", "genre": "Electronic", "publisher": "Parlophone"},
			{"date": "2001", "genre": "Rock", "publisher": "EMI"},
		} {
			title := fmt.Sprintf("Idiotheque %d", n+1)
			tags["artist"], tags["album"], tags["title"], tags["track"] = "Radiohead", "Kid A", title, strconv.Itoa(n+1)
			s.Uploaded(alice, s.UploadAudioFile(audiofile.Generate(t, title+".mp3", audiofile.Spec{Seconds: 2, Tags: tags})))
		}
		s.Navidrome.IndexedTrack(t, account, s.PersonalPath(alice, ""), "Idiotheque 3")
		s.ResolveSongs()
		c := s.Catalog(alice)
		results := s.Search(alice, "kid a", "").Results
		require.NotEmpty(t, results)

		chosen := s.Choose(alice, results[0])
		s.Open(alice, c.MusicButton(), c.MineTab(), c.AlbumsMode(), "💿 Radiohead — Kid A · 3 трека")

		assert.Equal(t, "2000 · 3 трека · 00:06 · Electronic", results[0].Description)
		facts := "💿 <b>Radiohead</b> — Kid A\n📅 2000 · 3 трека · 00:06\n🎼 Electronic · 🏷 Parlophone\n"
		edits := s.Telegram.InlineEdits(t, chosen.ChosenInlineResult.InlineMessageID)
		require.Len(t, edits, 1)
		assert.Contains(t, edits[0].Text, facts+"🔗")
		assert.Contains(t, s.WindowText(), facts+"📚")
	})
}

func TestEnrichTracks(t *testing.T) {
	t.Parallel()

	t.Run("track from before takes genres and label from its file", func(t *testing.T) {
		s := harness.New(t)
		s.Uploaded(alice, s.UploadAudioFile(paranoidAndroid(t)))
		s.ForgetGenres()

		s.Enrich()

		c := s.Catalog(alice)
		s.Open(alice, c.MusicButton(), c.MineTab(), "Radiohead — Paranoid Android")
		assert.Contains(t, s.WindowText(), "\n🎼 Alternative Rock, Art Rock · 🏷 Parlophone\n")
	})

	t.Run("tracks from before ask their provider, the shared one for its author", func(t *testing.T) {
		s := harness.New(t)
		importZvukRiffs(s)
		s.ShareTrack(alice, "Zvuk Band — Riff")
		s.ForgetGenres()
		s.Zvuk.AddTrack(zvuk.Track{
			ID: "701", Title: "Riff", Artists: []string{"Zvuk Band"}, Release: "700", Position: 1,
			Seconds: 2, Audio: s.ZvukAudio(), Genres: []string{"Metal"},
		})

		s.Enrich()

		c := s.Catalog(alice)
		s.Open(alice, c.MusicButton(), c.MineTab(), "Zvuk Band — Riff")
		assert.Contains(t, s.WindowText(), "\n🎼 Metal · 🏷 Zvuk Records\n")
		s.OpenShared(bob, 1)
		assert.Contains(t, s.WindowText(), "\n🎼 Metal · 🏷 Zvuk Records\n")
	})

	t.Run("track the provider failed for waits for the next run", func(t *testing.T) {
		s := harness.New(t)
		importZvukRiffs(s)
		s.ForgetGenres()
		s.Zvuk.SetDown(true)
		s.Enrich()
		c := s.Catalog(alice)
		s.Open(alice, c.MusicButton(), c.MineTab(), "Zvuk Band — Riff")
		failed := s.WindowText()
		s.Zvuk.SetDown(false)

		s.Enrich()

		assert.NotContains(t, failed, "🎼")
		s.Open(alice, c.MusicButton(), c.MineTab(), "Zvuk Band — Riff")
		assert.Contains(t, s.WindowText(), "\n🎼 Rock, Hard Rock · 🏷 Zvuk Records\n")
	})
}

// importZvukRiffs imports two of Alice's likes from "Zvuk Band — Riffs
// (2019)", a Pop release by Zvuk Records: "Riff" in Rock and Hard Rock, and
// "Ballad" without genres of its own.
func importZvukRiffs(s *harness.Scenario) {
	s.ConnectZvuk(alice, harness.ZvukToken)
	s.Zvuk.AddRelease(zvuk.Release{
		ID: "700", Title: "Riffs", Artists: []string{"Zvuk Band"}, Date: "2019-03-01", Genres: []string{"Pop"}, Label: "Zvuk Records",
	})
	s.Zvuk.AddTrack(zvuk.Track{
		ID: "701", Title: "Riff", Artists: []string{"Zvuk Band"}, Release: "700", Position: 1,
		Seconds: 2, Audio: s.ZvukAudio(), Genres: []string{"Rock", "Hard Rock"},
	})
	s.Zvuk.AddTrack(zvuk.Track{
		ID: "702", Title: "Ballad", Artists: []string{"Zvuk Band"}, Release: "700", Position: 2, Seconds: 2, Audio: s.ZvukAudio(),
	})
	s.LikeOnZvuk("701", "702")
	s.ImportZvuk(alice)
	s.WaitIngest()
}

func paranoidAndroid(t *testing.T) string {
	t.Helper()
	return audiofile.Generate(t, "paranoid.flac", audiofile.Spec{Seconds: 2, Tags: map[string]string{
		"artist": "Radiohead", "album": "OK Computer", "title": "Paranoid Android", "track": "2", "date": "1997",
		"genre": "Alternative Rock;Art Rock;Britpop", "publisher": "Parlophone",
	}})
}
