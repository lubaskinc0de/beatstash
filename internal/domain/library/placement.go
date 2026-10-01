// Placement: how the bot treats a Navidrome library by where it lies
// against the bot's folders in music_dir. SystemFolders.Reserved are folder
// names inside Root besides the Libraries'.

package library

import (
	"path"
	"strings"
)

// Placement is a value object: where a Navidrome library lies against the
// bot's folders, as Navidrome sees them.
type Placement int

const (
	// PlacedApart: the library is an Attached Library, wherever it lies, as
	// long as it does not touch the bot's folders.
	PlacedApart Placement = iota
	// PlacedInside: the library is one the bot keeps, or lies in a folder
	// of the bot.
	PlacedInside
	// PlacedAround: the library holds a folder of the bot. It is not an
	// Attached Library. Navidrome would index the bot's Tracks twice.
	PlacedAround
)

// SystemFolders is a value object: where the bot's Libraries lie in
// music_dir, as Navidrome sees it, and which other folders of the bot there
// no Navidrome library may take.
type SystemFolders struct {
	Root     string
	Reserved []string
}

// The bot writes its Libraries to these folders of music_dir only; the rest
// of it may hold Attached Libraries.
const (
	SharedLibraryDir     = "shared"
	PersonalLibrariesDir = "users"
)

// PlacementOf treats music_dir/users itself as Around: it is no library of
// the bot but holds all the Personal Libraries.
func PlacementOf(libraryPath string, folders SystemFolders) Placement {
	lib := path.Clean(libraryPath)
	shared, personal := folders.shared(), folders.personal()
	switch {
	case inOrUnder(lib, shared) || under(lib, personal):
		return PlacedInside
	case lib == personal || under(shared, lib) || under(personal, lib):
		return PlacedAround
	}
	for _, dir := range folders.reserved() {
		switch {
		case inOrUnder(lib, dir):
			return PlacedInside
		case under(dir, lib):
			return PlacedAround
		}
	}
	return PlacedApart
}

func (f SystemFolders) shared() string {
	return path.Join(f.Root, SharedLibraryDir)
}

func (f SystemFolders) personal() string {
	return path.Join(f.Root, PersonalLibrariesDir)
}

func (f SystemFolders) reserved() []string {
	dirs := make([]string, 0, len(f.Reserved))
	for _, name := range f.Reserved {
		dirs = append(dirs, path.Join(f.Root, name))
	}
	return dirs
}

func inOrUnder(p, dir string) bool {
	return p == dir || under(p, dir)
}

// under means strictly below dir.
func under(p, dir string) bool {
	return dir == "/" || strings.HasPrefix(p, dir+"/")
}
