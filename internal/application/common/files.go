package common

import "io"

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
	// Stage copies audio to a scratch file on the libraries' file system.
	Stage(audio io.Reader, ext string) (string, error)
	ClearScratch() error
}
