package harness

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/beatstash/e2e/harness/audiofile"
	"github.com/lubaskinc0de/beatstash/internal/domain/library"
	"github.com/lubaskinc0de/beatstash/internal/infra/disk"
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
	return filesUnder(s.t, s.SharedPath(""))
}

// FilesWithContent maps the files under root to their bytes.
func FilesWithContent(t *testing.T, root string) map[string]string {
	t.Helper()

	files := map[string]string{}
	for _, rel := range filesUnder(t, root) {
		data, err := os.ReadFile(filepath.Join(root, rel)) //nolint:gosec // G304: paths come from the scenario
		require.NoError(t, err)
		files[rel] = string(data)
	}
	return files
}

func FileSize(t *testing.T, path string) int64 {
	t.Helper()

	info, err := os.Stat(path)
	require.NoError(t, err)
	return info.Size()
}

// PutScratchFile leaves a file in the bot's scratch folder as if written
// age ago.
func (s *Scenario) PutScratchFile(name string, age time.Duration) {
	s.t.Helper()

	path := filepath.Join(s.scratchDir(), name)
	writeFile(s.t, path, []byte("left over"))
	written := time.Now().Add(-age)
	require.NoError(s.t, os.Chtimes(path, written, written))
}

func (s *Scenario) ScratchFiles() []string {
	s.t.Helper()
	return filesUnder(s.t, s.scratchDir())
}

func (s *Scenario) scratchDir() string {
	return filepath.Join(s.Library, disk.ScratchDir)
}

// The ByHand methods change a Library's files behind the bot's back, as
// the owner of the server might.
func (s *Scenario) RemoveByHand(path string) {
	s.t.Helper()
	require.NoError(s.t, os.Remove(path))
}

// WriteByHand writes the source's bytes to path. An existing file keeps
// its inode: its hardlinks change too.
func (s *Scenario) WriteByHand(path, source string) {
	s.t.Helper()
	copyFile(s.t, source, path)
}

func (s *Scenario) MoveByHand(from, to string) {
	s.t.Helper()

	require.NoError(s.t, os.MkdirAll(filepath.Dir(to), 0o755)) //nolint:gosec // G301: Navidrome container reads the library
	require.NoError(s.t, os.Rename(from, to))
}

func (s *Scenario) SharedPath(rel string) string {
	return filepath.Join(s.Library, library.SharedLibraryDir, rel)
}

// copyFile makes the folders of target as needed; an existing target keeps
// its inode.
func copyFile(t *testing.T, source, target string) {
	t.Helper()

	data, err := os.ReadFile(source) //nolint:gosec // G304: paths come from the scenario
	require.NoError(t, err)
	writeFile(t, target, data)
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755)) //nolint:gosec // G301: Navidrome container reads the library
	require.NoError(t, os.WriteFile(path, data, 0o644))        //nolint:gosec // G306: Navidrome container reads the library
}

func (s *Scenario) RetagByHand(path string, tags map[string]string) {
	s.t.Helper()
	audiofile.Retag(s.t, path, tags)
}
