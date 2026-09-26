package library

import "strings"

// Metadata is a value object: artist, title, album and the like.
type Metadata struct {
	AlbumArtist string
	Artist      string
	Album       string
	Title       string
	Year        int
	TrackNumber int
}

func (m Metadata) Merge(sources ...Metadata) Metadata {
	for _, s := range sources {
		m.AlbumArtist = firstString(m.AlbumArtist, s.AlbumArtist)
		m.Artist = firstString(m.Artist, s.Artist)
		m.Album = firstString(m.Album, s.Album)
		m.Title = firstString(m.Title, s.Title)
		m.Year = firstInt(m.Year, s.Year)
		m.TrackNumber = firstInt(m.TrackNumber, s.TrackNumber)
	}
	return m
}

func (m Metadata) Normalize() Metadata {
	m.AlbumArtist = collapseSpaces(m.AlbumArtist)
	m.Artist = collapseSpaces(m.Artist)
	m.Album = collapseSpaces(m.Album)
	m.Title = collapseSpaces(m.Title)
	if m.AlbumArtist == "" {
		m.AlbumArtist = m.Artist
	}
	return m
}

func (m Metadata) Complete() bool {
	return m.Artist != "" && m.Title != ""
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
