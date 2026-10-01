package common

import (
	"io"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

type Disk interface {
	// FreePath adds a " (N)" suffix if rel is taken in dir.
	FreePath(dir, rel string) (string, error)
	// Link hardlinks: a copy costs no space.
	Link(from, to string) error
	// Place renames atomically: Navidrome never sees a half-written file.
	Place(from, to string) error
	// Remove and Move run after the database changed, so they only log failures.
	Remove(path string)
	Move(from, to string)
	MakeDir(dir string) error
	// Stat leaves the Path of the file as given.
	Stat(path string) (library.LibraryFile, error)
	// ListFiles lists the files under dir, apart from hidden ones, with
	// paths relative to dir.
	ListFiles(dir string) ([]library.LibraryFile, error)
	// StatFiles looks at the paths relative to dir again; the ones gone
	// are left out.
	StatFiles(dir string, paths []string) ([]library.LibraryFile, error)
	// FreeSpace is how many bytes the libraries file system has left.
	FreeSpace() (int64, error)
	// Stage copies audio to a scratch file on the libraries' file system.
	Stage(audio io.Reader, ext string) (string, error)
	// Park keeps a staged file until the commit under a name of its own.
	Park(staged string) (string, error)
}
