// LayoutPath: the path a Track's metadata gives it inside a Library.

package library

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	inboxDir   = "Inbox"
	singlesDir = "Singles"

	// Leaves room for a " (2)" suffix and an extension within the 255-byte
	// file name limit.
	maxComponentBytes = 200
)

// LayoutPath returns the file's path in a Library:
//   - "Album Artist/Album (Year)/NN - Title" for an album track;
//   - "Artist/Singles/Title" for a single;
//   - "Inbox/<original name>" when artist or title is missing.
func LayoutPath(m Metadata, format Format, originalName string) string {
	ext := format.Ext()

	switch {
	case !m.Complete():
		stem := strings.TrimSuffix(originalName, filepath.Ext(originalName))
		return filepath.Join(inboxDir, sanitize(stem)+ext)
	case m.Album == "":
		return filepath.Join(sanitize(m.Artist), singlesDir, sanitize(m.Title)+ext)
	}

	album := m.Album
	if m.Year > 0 {
		album = fmt.Sprintf("%s (%d)", m.Album, m.Year)
	}
	name := m.Title
	if m.TrackNumber > 0 {
		name = fmt.Sprintf("%02d - %s", m.TrackNumber, m.Title)
	}
	return filepath.Join(sanitize(m.AlbumArtist), sanitize(album), sanitize(name)+ext)
}

// sanitize turns any string into one path component. It replaces path
// separators and characters Windows does not allow, and trims dots and
// spaces at the ends: a leading dot would hide the file from Navidrome.
func sanitize(s string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case strings.ContainsRune(`/\:*?"<>|`, r):
			return '_'
		case unicode.IsControl(r):
			return -1
		}
		return r
	}, s)

	s = trimEnds(s)
	if len(s) > maxComponentBytes {
		cut := maxComponentBytes
		for cut > 0 && !utf8.RuneStart(s[cut]) {
			cut--
		}
		s = trimEnds(s[:cut])
	}

	if s == "" {
		return "_"
	}
	return s
}

func trimEnds(s string) string {
	s = strings.TrimLeft(s, ". ")
	return strings.TrimRight(s, ". ")
}
