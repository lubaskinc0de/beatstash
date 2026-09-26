package harness

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
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

// PersonalDir is where the bot keeps the user's Personal Library, relative
// to music_dir.
func (s *Scenario) PersonalDir(user User) string {
	s.t.Helper()

	conn, err := pgx.Connect(s.t.Context(), s.config.DBDSN)
	require.NoError(s.t, err)
	defer conn.Close(context.Background())

	var dir string
	err = conn.QueryRow(s.t.Context(), `
		SELECT libraries.dir FROM libraries
		JOIN identities ON identities.user_id = libraries.owner_id
		WHERE identities.channel = 'telegram' AND identities.external_id = $1`,
		strconv.FormatInt(user.ID, 10),
	).Scan(&dir)
	require.NoError(s.t, err, "no Personal Library of %s", user.Username)
	return dir
}

func (s *Scenario) PersonalFiles(user User) []string {
	s.t.Helper()
	return filesUnder(s.t, filepath.Join(s.Library, s.PersonalDir(user)))
}

func (s *Scenario) PersonalPath(user User, rel string) string {
	s.t.Helper()
	return filepath.Join(s.Library, s.PersonalDir(user), rel)
}

func (s *Scenario) SharedFiles() []string {
	s.t.Helper()
	return filesUnder(s.t, filepath.Join(s.Library, "shared"))
}
