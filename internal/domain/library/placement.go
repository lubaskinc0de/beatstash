package library

import (
	"path"
	"strings"
)

// Placement is a value object: where a Navidrome library lies against
// music_dir, both as Navidrome sees them.
type Placement int

const (
	// PlacedApart: the library is an Attached Library.
	PlacedApart Placement = iota
	// PlacedInside: the library is one the bot keeps.
	PlacedInside
	// PlacedAround: the library holds music_dir. It is no Attached Library:
	// every Track of the bot would have a second Track in it.
	PlacedAround
)

func PlacementOf(libraryPath, musicDir string) Placement {
	lib, root := path.Clean(libraryPath), path.Clean(musicDir)
	switch {
	case lib == root || within(root, lib):
		return PlacedAround
	case within(lib, root):
		return PlacedInside
	}
	return PlacedApart
}

func within(p, dir string) bool {
	return dir == "/" || strings.HasPrefix(p, dir+"/")
}
