package library

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// FreePath returns rel, or rel with a " (N)" suffix if a file already
// takes its place.
func FreePath(library, rel string) (string, error) {
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

// Link gives the file a second name. Libraries share one file system, so
// a copy costs no space; replacing either file later breaks the link.
func Link(from, to string) error {
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	return os.Link(from, to)
}

// RemoveFile runs once the database has already changed, so a failure
// can only be logged.
func RemoveFile(path string) {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		slog.Error("library_remove_failed", "path", path, "error", err)
	}
}

// Place moves the staged file into the Library in one atomic rename, so
// Navidrome never sees a half-written file.
func Place(staged, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.Rename(staged, target)
}

// MoveFile runs once the database has already changed, so a failure can
// only be logged.
func MoveFile(from, to string) {
	if err := os.Rename(from, to); err != nil {
		slog.Error("library_move_failed", "from", from, "to", to, "error", err)
	}
}

// FileChanges holds what makes the Library follow a transaction: files
// change before the commit, so a rollback has to take them back, and old
// files go only after the commit.
type FileChanges struct {
	onRollback  []func()
	afterCommit []func()
}

func (c *FileChanges) OnRollback(undo func()) {
	c.onRollback = append(c.onRollback, undo)
}

func (c *FileChanges) AfterCommit(finish func()) {
	c.afterCommit = append(c.afterCommit, finish)
}

// Settle takes the changes back if the transaction failed with err, and
// finishes them otherwise.
func (c *FileChanges) Settle(err error) {
	steps := c.afterCommit
	if err != nil {
		steps = c.onRollback
	}
	for _, step := range steps {
		step()
	}
	c.onRollback, c.afterCommit = nil, nil
}
