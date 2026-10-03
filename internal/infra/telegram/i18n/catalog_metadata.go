package i18n

import (
	"strconv"
	"strings"

	"github.com/lubaskinc0de/beatstash/internal/application/common/repositories"
	"github.com/lubaskinc0de/beatstash/internal/domain/library"
)

// maxGenres keeps a description within the line or two Telegram shows.
const maxGenres = 2

// TrackDescription is plain text.
func TrackDescription(track *library.Track) string {
	album := ""
	if track.Album != "" {
		album = "💿 " + track.Album
	}
	return joinParts(" · ", album, yearText(track.Year), genreList(track.Genres), durationText(track.DurationMs))
}

// AlbumDescription is plain text.
func (c Catalog) AlbumDescription(album repositories.AlbumSummary) string {
	return joinParts(" · ", yearText(album.Year), c.tracks(album.Tracks), durationText(album.DurationMs), genreList(album.Genres))
}

// AlbumText tells the Album with its tracks counted.
func (c Catalog) AlbumText(album repositories.AlbumSummary) string {
	return joinParts("\n", AlbumCaption(album.AlbumArtist, album.Album), c.albumFacts(album, album.Tracks))
}

// albumFacts leaves out the count of tracks for none: a single has no
// Album.
func (c Catalog) albumFacts(album repositories.AlbumSummary, tracks int) string {
	year, count := "", ""
	if album.Year > 0 {
		year = "📅 " + yearText(album.Year)
	}
	if tracks > 0 {
		count = c.tracks(tracks)
	}
	return joinParts("\n", joinParts(" · ", year, count, durationText(album.DurationMs)), genresAndLabel(album.Genres, album.Label))
}

// AudioCaption goes under the Track's audio: the player shows the artist
// and the title.
func (c Catalog) AudioCaption(track *library.Track) string {
	var album string
	switch {
	case track.Album != "" && track.Year > 0:
		album = "💿 " + esc(track.Album) + " (" + yearText(track.Year) + ")"
	case track.Album != "":
		album = "💿 " + esc(track.Album)
	case track.Year > 0:
		album = "📅 " + yearText(track.Year)
	}
	return joinParts("\n", album, genresAndLabel(track.Genres, track.Label))
}

// TrackText tells the Track where no player shows its artist and title.
func (c Catalog) TrackText(track *library.Track) string {
	return joinParts("\n", TrackCaption(track.Artist, track.Title), c.AudioCaption(track))
}

func genresAndLabel(genres []string, label string) string {
	var genre, labelText string
	if list := genreList(genres); list != "" {
		genre = "🎼 " + esc(list)
	}
	if label != "" {
		labelText = "🏷 " + esc(label)
	}
	return joinParts(" · ", genre, labelText)
}

func genreList(genres []string) string {
	return strings.Join(genres[:min(len(genres), maxGenres)], ", ")
}

func durationText(ms int) string {
	if ms <= 0 {
		return ""
	}
	return formatSeconds(ms / 1000)
}

func yearText(year int) string {
	if year <= 0 {
		return ""
	}
	return strconv.Itoa(year)
}

func joinParts(sep string, parts ...string) string {
	kept := parts[:0]
	for _, part := range parts {
		if part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, sep)
}
