// Library: a tree of audio files Navidrome indexes, its kind, owner and
// Quota, and which Navidrome libraries an account may be given. Dir is
// relative to music_dir; an Attached Library's Dir is its path as Navidrome
// sees it. NavidromeID is zero until the library is created in Navidrome. A
// Personal Library's Quota is nil while it follows the Default Quota.

package library

import (
	"errors"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
)

// Library is an aggregate root. It does not hold its Tracks: Tracks refer to
// it by LibraryID, so loading a Library never loads them.
type Library struct {
	ID uint `gorm:"primaryKey"`

	Kind        LibraryKind `gorm:"not null"`
	OwnerID     *uint       `gorm:"uniqueIndex"`
	Dir         string      `gorm:"not null;uniqueIndex"`
	NavidromeID int         `gorm:"not null;default:0"`
	Quota       *Quota

	CreatedAt time.Time
}

// LibraryKind is a value object: personal, shared or attached.
type LibraryKind string

const (
	LibraryPersonal LibraryKind = "personal"
	LibraryShared   LibraryKind = "shared"
	LibraryAttached LibraryKind = "attached"
)

var ErrNotPersonal = errors.New("only a Personal Library has a Quota of its own")

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

// Usage tells how much the Library, weighing weight bytes, takes of the
// Quota in force for it.
func (l *Library) Usage(weight int64, server ServerQuotas) Usage {
	return Usage{Used: weight, Quota: l.quotaIn(server)}
}

// quotaIn returns the Quota in force: a Personal Library's own or else the
// server's Default Quota, the Shared Library's for it. An Attached Library
// has none: its files are not the bot's.
func (l *Library) quotaIn(server ServerQuotas) Quota {
	switch {
	case l.Kind == LibraryPersonal && l.Quota != nil:
		return *l.Quota
	case l.Kind == LibraryPersonal:
		return server.Default
	case l.Kind == LibraryShared:
		return server.Shared
	default:
		return Unlimited
	}
}

// SetQuota takes nil to follow the Default Quota again.
func (l *Library) SetQuota(q *Quota) error {
	if l.Kind != LibraryPersonal {
		return ErrNotPersonal
	}
	l.Quota = q
	return nil
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
func Grant(current []int, all []Library, paths map[int]string, folders SystemFolders, personal, shared *Library) []int {
	others := map[int]bool{}
	for _, lib := range all {
		if lib.Kind == LibraryPersonal && lib.ID != personal.ID {
			others[lib.NavidromeID] = true
		}
	}
	ids := []int{personal.NavidromeID, shared.NavidromeID}
	for _, id := range current {
		around := PlacementOf(paths[id], folders) == PlacedAround
		if !others[id] && !around && !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	return ids
}
