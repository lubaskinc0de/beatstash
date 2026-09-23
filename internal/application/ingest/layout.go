package ingest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

const (
	inboxDir   = "Inbox"
	singlesDir = "Singles"

	// Leaves room for a collision suffix and an extension within the
	// 255-byte file name limit.
	maxComponentBytes = 200
)

func layoutPath(m domain.Metadata, format domain.Format, originalName string) string {
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

// sanitize makes a single path component out of any string: no separators
// or characters Windows clients choke on, no leading dots that would hide
// the entry from Navidrome, no trailing dots or spaces.
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

// freePath returns rel, or rel with a " (N)" suffix if a file already
// takes its place.
func freePath(library, rel string) (string, error) {
	ext := filepath.Ext(rel)
	stem := strings.TrimSuffix(rel, ext)

	candidate := rel
	for n := 2; ; n++ {
		_, err := os.Stat(filepath.Join(library, candidate))
		if errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		}
		if err != nil {
			return "", err
		}
		candidate = fmt.Sprintf("%s (%d)%s", stem, n, ext)
	}
}
