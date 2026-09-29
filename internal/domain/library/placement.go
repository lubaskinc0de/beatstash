package library

import (
	"path"
	"strings"
)

// Placement is a value object: where a Navidrome library lies against the
// folders of music_dir the bot writes to, as Navidrome sees them.
type Placement int

const (
	// PlacedApart: the library is an Attached Library, wherever it lies, as
	// long as it does not touch the bot's folders.
	PlacedApart Placement = iota
	// PlacedInside: the library is one the bot keeps.
	PlacedInside
	// PlacedAround: the library holds a folder of the bot. It is not an
	// Attached Library. Navidrome would index the bot's Tracks twice.
	PlacedAround
)

// PlacementOf treats music_dir/users itself as Around: it is no library of
// the bot but holds all the Personal Libraries.
func PlacementOf(libraryPath, musicDir string) Placement {
	lib, root := path.Clean(libraryPath), path.Clean(musicDir)
	shared := path.Join(root, SharedLibraryDir)
	personal := path.Join(root, PersonalLibrariesDir)
	scratch := path.Join(root, ScratchDir)
	switch {
	case inOrUnder(lib, shared) || under(lib, personal) || inOrUnder(lib, scratch):
		return PlacedInside
	case lib == personal || under(shared, lib) || under(personal, lib) || under(scratch, lib):
		return PlacedAround
	}
	return PlacedApart
}

func inOrUnder(p, dir string) bool {
	return p == dir || under(p, dir)
}

// under means strictly below dir.
func under(p, dir string) bool {
	return dir == "/" || strings.HasPrefix(p, dir+"/")
}
