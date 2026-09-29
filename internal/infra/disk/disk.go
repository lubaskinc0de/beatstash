package disk

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

type Disk struct {
	MusicDir string
}

func (d *Disk) FreePath(dir, rel string) (string, error) {
	ext := filepath.Ext(rel)
	stem := strings.TrimSuffix(rel, ext)

	candidate := rel
	for n := 2; ; n++ {
		_, err := os.Stat(filepath.Join(dir, candidate))
		if errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		}
		if err != nil {
			return "", err
		}
		candidate = fmt.Sprintf("%s (%d)%s", stem, n, ext)
	}
}

func (d *Disk) Link(from, to string) error {
	if err := d.MakeDir(filepath.Dir(to)); err != nil {
		return err
	}
	return os.Link(from, to)
}

func (d *Disk) Place(from, to string) error {
	if err := d.MakeDir(filepath.Dir(to)); err != nil {
		return err
	}
	return os.Rename(from, to)
}

func (d *Disk) Remove(path string) {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		slog.Error("library_remove_failed", "path", path, "error", err)
	}
}

func (d *Disk) Move(from, to string) {
	if err := os.Rename(from, to); err != nil {
		slog.Error("library_move_failed", "from", from, "to", to, "error", err)
	}
}

func (d *Disk) MakeDir(dir string) error {
	return os.MkdirAll(dir, 0o755) //nolint:gosec // G301: Navidrome reads the library
}

func (d *Disk) Stage(audio io.Reader, ext string) (string, error) {
	dir := filepath.Join(d.MusicDir, library.ScratchDir)
	if err := d.MakeDir(dir); err != nil {
		return "", err
	}

	file, err := os.CreateTemp(dir, "ingest-*"+ext)
	if err != nil {
		return "", err
	}

	_, copyErr := io.Copy(file, audio)
	closeErr := file.Close()
	if err := errors.Join(copyErr, closeErr); err != nil {
		_ = os.Remove(file.Name())
		return "", fmt.Errorf("copy audio: %w", err)
	}
	return file.Name(), nil
}

func (d *Disk) ClearScratch() error {
	return os.RemoveAll(filepath.Join(d.MusicDir, library.ScratchDir))
}
