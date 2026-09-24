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
