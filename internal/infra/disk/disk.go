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
	"syscall"
	"time"

	"github.com/lubaskinc0de/beatstash/internal/domain/library"
)

// ScratchDir is the folder of music_dir that holds files on their way into
// a Library. Navidrome skips hidden folders.
const ScratchDir = ".beatstash"

type Disk struct {
	MusicDir string
	// Scratch is where files wait to go into a Library. Several instances
	// share it, so it is never cleared whole.
	Scratch string
	// ScratchTTL is how old a scratch file gets before it counts as left
	// over. A download in progress writes to its file all the time.
	ScratchTTL time.Duration
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

func (d *Disk) Stat(path string) (library.LibraryFile, error) {
	info, err := os.Stat(path)
	if err != nil {
		return library.LibraryFile{}, err
	}
	return libraryFile(path, info), nil
}

func (d *Disk) ListFiles(dir string) ([]library.LibraryFile, error) {
	var files []library.LibraryFile
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasPrefix(entry.Name(), ".") && path != dir {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files = append(files, libraryFile(rel, info))
		return nil
	})
	return files, err
}

func (d *Disk) StatFiles(dir string, paths []string) ([]library.LibraryFile, error) {
	files := make([]library.LibraryFile, 0, len(paths))
	for _, rel := range paths {
		info, err := os.Stat(filepath.Join(dir, rel))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		files = append(files, libraryFile(rel, info))
	}
	return files, nil
}

func libraryFile(path string, info fs.FileInfo) library.LibraryFile {
	file := library.LibraryFile{Path: path, Size: info.Size(), ModTime: info.ModTime()}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		file.Inode = stat.Ino
	}
	return file
}

func (d *Disk) FreeSpace() (int64, error) {
	var fs syscall.Statfs_t
	if err := syscall.Statfs(d.MusicDir, &fs); err != nil {
		return 0, err
	}
	return int64(fs.Bavail) * fs.Bsize, nil //nolint:gosec // G115: no disk holds 2^63 bytes
}

func (d *Disk) Stage(audio io.Reader, ext string) (string, error) {
	if err := d.MakeDir(d.Scratch); err != nil {
		return "", err
	}

	file, err := os.CreateTemp(d.Scratch, "ingest-*"+ext)
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

// Park moves the staged file to a name of its own in the scratch folder.
func (d *Disk) Park(staged string) (string, error) {
	file, err := os.CreateTemp(d.Scratch, "parked-*"+filepath.Ext(staged))
	if err != nil {
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(staged, file.Name()); err != nil {
		_ = os.Remove(file.Name())
		return "", err
	}
	return file.Name(), nil
}

// SweepScratch removes the scratch files older than ScratchTTL: those of
// Ingests that crashed. Folders stay: the bot makes none there.
func (d *Disk) SweepScratch() {
	entries, err := os.ReadDir(d.Scratch)
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	if err != nil {
		slog.Error("sweep_scratch", "error", err)
		return
	}
	cutoff := time.Now().Add(-d.ScratchTTL)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		d.Remove(filepath.Join(d.Scratch, entry.Name()))
	}
}
