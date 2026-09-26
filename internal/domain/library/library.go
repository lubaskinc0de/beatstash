package library

import (
	"path/filepath"
	"strconv"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
)

const (
	SharedLibraryDir     = "shared"
	PersonalLibrariesDir = "users"
)

// Library is an aggregate root. It does not hold its Tracks: Tracks refer to
// it by LibraryID, so loading a Library never loads them.
type Library struct {
	ID uint `gorm:"primaryKey"`

	Kind    LibraryKind `gorm:"not null"`
	OwnerID *uint       `gorm:"uniqueIndex"`
	// Dir is relative to music_dir.
	Dir string `gorm:"not null;uniqueIndex"`
	// NavidromeID is zero until the library is created in Navidrome.
	NavidromeID int `gorm:"not null;default:0"`

	CreatedAt time.Time
}

// LibraryKind is a value object: personal or shared.
type LibraryKind string

const (
	LibraryPersonal LibraryKind = "personal"
	LibraryShared   LibraryKind = "shared"
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

func (l *Library) LinkNavidrome(id int) {
	l.NavidromeID = id
}
