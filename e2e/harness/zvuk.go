package harness

import (
	"fmt"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/assert"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/zvuk"
)

const ZvukToken = "zvuk-token-alice" //nolint:gosec // G101: test-only token

// ConnectZvuk creates the Zvuk account anew, with a subscription and an
// empty collection, and connects it to the bot.
func (s *Scenario) ConnectZvuk(user User, token string) *zvuk.Account {
	s.t.Helper()

	account := s.Zvuk.AddAccount(token, true)
	s.SendZvukToken(user, token)
	assert.Contains(s.t, s.WindowText(), "Звук подключён")
	return account
}

func (s *Scenario) SendZvukToken(user User, token string) *models.Update {
	s.t.Helper()

	s.OpenZvuk(user, "🔌 Подключить")
	return s.SendText(user, token)
}

func (s *Scenario) OpenZvuk(user User, buttons ...string) {
	s.t.Helper()

	s.Open(user, "📥 Импорт из музыкального сервиса", "🟣 Звук")
	s.Go(user, buttons...)
}

type ZvukCollection struct {
	Liked    []string
	Album    []string
	Playlist []string
	// Tracks lists each track of the collection once.
	Tracks []string
}

// AddZvukCollection saves into the account 5 liked tracks, an album of 3
// and a playlist of 2, one of them liked: 9 tracks in all.
func (s *Scenario) AddZvukCollection(token string) ZvukCollection {
	s.t.Helper()

	var c ZvukCollection
	c.Liked = s.AddZvukAlbum("620", "Liked", 5)
	c.Album = s.AddZvukAlbum("610", "Saved", 3)
	listed := s.AddZvukAlbum("630", "Listed", 1)
	c.Playlist = []string{listed[0], c.Liked[0]}
	s.Zvuk.AddPlaylist(zvuk.Playlist{ID: "810", Title: "My Playlist", Tracks: c.Playlist})
	s.Zvuk.Update(token, func(a *zvuk.Account) {
		a.Liked = c.Liked
		a.Releases = []string{"610"}
		a.Playlists = []string{"810"}
	})

	c.Tracks = append(append(append(c.Tracks, c.Liked...), c.Album...), listed[0])
	return c
}

func (s *Scenario) ImportZvuk(user User) {
	s.t.Helper()

	s.OpenZvuk(user, "📥 Импортировать", "▶️ Начать")
}

func (s *Scenario) LikeOnZvuk(ids ...string) {
	s.t.Helper()
	s.Zvuk.Update(ZvukToken, func(a *zvuk.Account) { a.Liked = ids })
}

// AddZvukSong's tracks all belong to "Zvuk Band — Zvuk Album (2021)".
func (s *Scenario) AddZvukSong(id, title string, hasFlac bool) {
	s.t.Helper()

	s.Zvuk.AddRelease(zvuk.Release{ID: "500", Title: "Zvuk Album", Artists: []string{"Zvuk Band"}, Date: "2021-01-01"})
	s.Zvuk.AddTrack(zvuk.Track{
		ID: id, Title: title, Artists: []string{"Zvuk Band"}, Release: "500", Position: 1,
		Seconds: 2, HasFlac: hasFlac, Audio: s.ZvukAudio(),
	})
}

// ZvukAudio shares one set of files among the scenario's tracks: Zvuk's
// metadata tells them apart.
func (s *Scenario) ZvukAudio() zvuk.Audio {
	if s.sharedZvukAudio == (zvuk.Audio{}) {
		s.sharedZvukAudio = zvuk.NewAudio(s.t, 2)
	}
	return s.sharedZvukAudio
}

// AddZvukAlbum adds "Zvuk Band — title (2020)" with n MP3-only tracks
// numbered from 1, their ids made of the album's.
func (s *Scenario) AddZvukAlbum(id, title string, n int) []string {
	s.t.Helper()

	s.Zvuk.AddRelease(zvuk.Release{ID: id, Title: title, Artists: []string{"Zvuk Band"}, Date: "2020-02-02"})
	var ids []string
	for i := 1; i <= n; i++ {
		trackID := fmt.Sprintf("%s%02d", id, i)
		s.Zvuk.AddTrack(zvuk.Track{
			ID: trackID, Title: fmt.Sprintf("Song %d", i), Artists: []string{"Zvuk Band"}, Release: id, Position: i,
			Seconds: 2, Audio: s.ZvukAudio(),
		})
		ids = append(ids, trackID)
	}
	return ids
}
