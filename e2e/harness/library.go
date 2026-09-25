package harness

import (
	"errors"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Hidden entries are the bot's scratch space.
func (s *Scenario) LibraryFiles() []string {
	s.t.Helper()
	return filesUnder(s.t, s.Library)
}

func filesUnder(t *testing.T, root string) []string {
	t.Helper()

	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if errors.Is(err, fs.ErrNotExist) && path == root {
			return filepath.SkipDir
		}
		if err != nil {
			return err
		}
		if strings.HasPrefix(d.Name(), ".") && path != root {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, rel)
		return nil
	})
	require.NoError(t, err)
	slices.Sort(files)
	return files
}

func PersonalDir(user User) string {
	return filepath.Join("users", strconv.FormatInt(user.ID, 10))
}

func (s *Scenario) PersonalFiles(user User) []string {
	s.t.Helper()
	return filesUnder(s.t, filepath.Join(s.Library, PersonalDir(user)))
}

func (s *Scenario) PersonalPath(user User, rel string) string {
	return filepath.Join(s.Library, PersonalDir(user), rel)
}

func (s *Scenario) SharedFiles() []string {
	s.t.Helper()
	return filesUnder(s.t, filepath.Join(s.Library, "shared"))
}
