package library

import (
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
)

// The bot writes to these folders of music_dir only; the rest of it may
// hold Attached Libraries.
const (
	SharedLibraryDir     = "shared"
	PersonalLibrariesDir = "users"
	// ScratchDir holds files on their way into a library. Navidrome skips
	// hidden directories.
	ScratchDir = ".navidrome-tg"
)

// Library is an aggregate root. It does not hold its Tracks: Tracks refer to
// it by LibraryID, so loading a Library never loads them.
type Library struct {
	ID uint `gorm:"primaryKey"`

	Kind    LibraryKind `gorm:"not null"`
	OwnerID *uint       `gorm:"uniqueIndex"`
	// Dir is relative to music_dir. An Attached Library lies outside
	// music_dir: its Dir is its path as Navidrome sees it.
	Dir string `gorm:"not null;uniqueIndex"`
	// NavidromeID is zero until the library is created in Navidrome.
	NavidromeID int `gorm:"not null;default:0"`

	CreatedAt time.Time
}

// LibraryKind is a value object: personal, shared or attached.
type LibraryKind string

const (
	LibraryPersonal LibraryKind = "personal"
	LibraryShared   LibraryKind = "shared"
	LibraryAttached LibraryKind = "attached"
)

func SharedLibrary() *Library {
	return &Library{Kind: LibraryShared, Dir: SharedLibraryDir}
}

func PersonalLibrary(owner *access.User) *Library {
	return &Library{
		Kind:    LibraryPersonal,
		OwnerID: &owner.ID,
		Dir:     filepath.Join(PersonalLibrariesDir, strconv.FormatUint(uint64(owner.ID), 10)),
	}
}

// AttachedLibrary takes a Navidrome library at path into account.
func AttachedLibrary(navidromeID int, path string) *Library {
	return &Library{Kind: LibraryAttached, Dir: path, NavidromeID: navidromeID}
}

func (l *Library) LinkNavidrome(id int) {
	l.NavidromeID = id
}

func (l *Library) Attached() bool {
	return l.Kind == LibraryAttached
}

// Follow brings the Tracks of an Attached Library in step with the songs
// Navidrome has indexed there. A Track follows its song by id. It returns
// the new and changed Tracks to save, and the Tracks whose songs are gone.
func (l *Library) Follow(tracks []Track, songs []Song) (save, gone []*Track) {
	indexed := make(map[string]bool, len(songs))
	for _, song := range songs {
		indexed[song.ID] = true
	}
	bySong := make(map[string]*Track, len(tracks))
	for n := range tracks {
		track := &tracks[n]
		if !indexed[track.SongID] {
			gone = append(gone, track)
			continue
		}
		bySong[track.SongID] = track
	}
	for _, song := range songs {
		track, ok := bySong[song.ID]
		switch {
		case !ok:
			save = append(save, NewAttachedTrack(l, song))
		case track.Follow(song):
			save = append(save, track)
		}
	}
	return save, gone
}

// MoveAttached follows an Attached Library whose path an admin changed in
// Navidrome.
func (l *Library) MoveAttached(path string) {
	l.Dir = path
}

// Grant returns the Navidrome libraries an account linked to the owner of
// personal may see: the ones it sees now, plus personal and shared, but
// none that shows other users' Personal Libraries: theirs, or one that holds
// the bot's folders. paths maps Navidrome's library ids to their paths.
func Grant(current []int, all []Library, paths map[int]string, musicDir string, personal, shared *Library) []int {
	others := map[int]bool{}
	for _, lib := range all {
		if lib.Kind == LibraryPersonal && lib.ID != personal.ID {
			others[lib.NavidromeID] = true
		}
	}
	ids := []int{personal.NavidromeID, shared.NavidromeID}
	for _, id := range current {
		around := PlacementOf(paths[id], musicDir) == PlacedAround
		if !others[id] && !around && !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	return ids
}
