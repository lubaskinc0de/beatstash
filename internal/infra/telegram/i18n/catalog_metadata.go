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
	return joinParts("\n\n", musicHeading("💿", album.AlbumArtist, album.Album), c.albumFacts(album, album.Tracks))
}

// albumFacts leaves out the count of tracks for none: a single has no
// Album.
func (c Catalog) albumFacts(album repositories.AlbumSummary, tracks int) string {
	var year, count, duration string
	if album.Year > 0 {
		year = "📅 " + c.t("metadata.year", args{"Year": album.Year})
	}
	if tracks > 0 {
		count = "🎵 " + c.t("metadata.tracks", args{"Tracks": tracks})
	}
	if album.DurationMs > 0 {
		duration = "⏱ " + c.t("metadata.duration", args{"Duration": durationText(album.DurationMs)})
	}
	return joinParts("\n", year, count, duration, c.genresAndLabel(album.Genres, album.Label))
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
	return joinParts("\n", album, c.genresAndLabel(track.Genres, track.Label))
}

// TrackText tells the Track where no player shows its artist and title.
func (c Catalog) TrackText(track *library.Track) string {
	return joinParts("\n\n", musicHeading("🎧", track.Artist, track.Title), c.AudioCaption(track))
}

func (c Catalog) genresAndLabel(genres []string, label string) string {
	var genre, labelText string
	if list := genreList(genres); list != "" {
		genre = "🎼 " + c.t("metadata.genres", args{"Genres": esc(list)})
	}
	if label != "" {
		labelText = "🏷 " + c.t("metadata.label", args{"Label": esc(label)})
	}
	return joinParts("\n", genre, labelText)
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

// musicHeading gives the title and artist separate lines in a full card.
func musicHeading(icon, artist, title string) string {
	return joinParts("\n", icon+" <b>"+esc(title)+"</b>", esc(artist))
}

// SharedTrackCaption also works when the player already shows the title.
func (c Catalog) SharedTrackCaption(caption string) string {
	return joinParts("\n\n", esc(c.t("share_message.track", nil)), caption)
}

func (c Catalog) SharedAlbumCaption(caption string) string {
	return joinParts("\n\n", esc(c.t("share_message.album", nil)), caption)
}
