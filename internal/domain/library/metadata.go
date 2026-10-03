// Metadata: the tags of a Track, merged from several sources and tidied so
// Duplicates match. Genres is nil until the sources were looked at for them:
// an empty slice means none had any.

package library

import (
	"slices"
	"strings"
)

// Metadata is a value object: artist, title, album and the like.
type Metadata struct {
	AlbumArtist string
	Artist      string
	Album       string
	Title       string
	Year        int
	TrackNumber int
	Genres      []string `gorm:"type:text[];serializer:pgarray"`
	Label       string   `gorm:"not null;default:''"`
}

func (m Metadata) Merge(sources ...Metadata) Metadata {
	for _, s := range sources {
		m.AlbumArtist = firstString(m.AlbumArtist, s.AlbumArtist)
		m.Artist = firstString(m.Artist, s.Artist)
		m.Album = firstString(m.Album, s.Album)
		m.Title = firstString(m.Title, s.Title)
		m.Year = firstInt(m.Year, s.Year)
		m.TrackNumber = firstInt(m.TrackNumber, s.TrackNumber)
		m = m.fillGenresAndLabel(s)
	}
	return m
}

func (m Metadata) Normalize() Metadata {
	m.AlbumArtist = collapseSpaces(m.AlbumArtist)
	m.Artist = collapseSpaces(m.Artist)
	m.Album = collapseSpaces(m.Album)
	m.Title = collapseSpaces(m.Title)
	m.Label = collapseSpaces(m.Label)
	if m.Genres != nil {
		genres := make([]string, 0, len(m.Genres))
		for _, genre := range m.Genres {
			if genre = collapseSpaces(genre); genre != "" && !slices.Contains(genres, genre) {
				genres = append(genres, genre)
			}
		}
		m.Genres = genres
	}
	if m.AlbumArtist == "" {
		m.AlbumArtist = m.Artist
	}
	return m
}

func (m Metadata) Complete() bool {
	return m.Artist != "" && m.Title != ""
}

// Equal tells looked-up Genres from not looked-up ones even when both are
// empty.
func (m Metadata) Equal(other Metadata) bool {
	return m.AlbumArtist == other.AlbumArtist && m.Artist == other.Artist && m.Album == other.Album &&
		m.Title == other.Title && m.Year == other.Year && m.TrackNumber == other.TrackNumber &&
		m.Label == other.Label && (m.Genres == nil) == (other.Genres == nil) && slices.Equal(m.Genres, other.Genres)
}

// fillGenresAndLabel keeps the Genres and the Label m has and takes the
// missing ones from the source.
func (m Metadata) fillGenresAndLabel(source Metadata) Metadata {
	if len(m.Genres) == 0 && (len(source.Genres) > 0 || m.Genres == nil) {
		m.Genres = slices.Clone(source.Genres)
	}
	m.Label = firstString(m.Label, source.Label)
	return m
}

func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func firstString(current, candidate string) string {
	if strings.TrimSpace(current) != "" {
		return current
	}
	return candidate
}

func firstInt(current, candidate int) int {
	if current > 0 {
		return current
	}
	return candidate
}
