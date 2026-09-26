package library

import (
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"

	"path/filepath"
	"strconv"
	"time"
)

const (
	SharedLibraryDir     = "shared"
	PersonalLibrariesDir = "users"
)

type LibraryKind string

const (
	LibraryPersonal LibraryKind = "personal"
	LibraryShared   LibraryKind = "shared"
)

type Library struct {
	ID uint `gorm:"primaryKey"`

	Kind    LibraryKind `gorm:"not null"`
	OwnerID *uint       `gorm:"uniqueIndex"`
	// Dir is relative to music_dir.
	Dir string `gorm:"not null;uniqueIndex"`
	// NavidromeID stays zero until the library is created in Navidrome.
	NavidromeID int `gorm:"not null;default:0"`

	CreatedAt time.Time
}

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
