package installer_test

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpgrade(t *testing.T) {
	t.Run("new release updates files, backs up and restarts", func(t *testing.T) {
		s := newSetup(t)
		s.install("")
		oldCompose, oldConfig := s.Read("compose.yml"), s.Read("config.toml")
		s.Version = "1.1.0"
		s.Compose = strings.Replace(composeTemplate, "command: [sleep, '86400']", "command: [sleep, '86401']", 1)
		s.Config = strings.Replace(s.Config, "[invites]\n", "[invites]\n# Added in 1.1.0.\nreminder = \"24h\"\n", 1)

		out, err := s.Run([]string{"upgrade"}, "", "y", "y", "y", "y")

		require.NoError(t, err, out)
		assert.Equal(t, s.Compose, s.Read("compose.yml"))
		assert.Equal(t, oldCompose, s.Read("compose.yml.bak"))
		assert.Equal(t, oldConfig, s.Read("config.toml.bak"))
		assert.Contains(t, s.Read("config.toml"), "[invites]\nttl = \"168h\"\n# Added in 1.1.0.\nreminder = \"24h\"\n")
		assert.Equal(t, []any{"telegram:42"}, s.Settings()["admins"])
		assert.Contains(t, s.Read(".env"), `BEATSTASH_VERSION="1.1.0"`)
		assert.Contains(t, dump(t, s.Deploy("backups")), "PostgreSQL database dump")
		assert.Contains(t, docker(t, "ps", "--filter", "label=com.docker.compose.project=beatstash", "--filter", "label=com.docker.compose.service=bot", "--format", "{{.Command}}"), "86401")
	})

	t.Run("same release has nothing to do", func(t *testing.T) {
		s := newSetup(t)
		s.install("")

		out, err := s.Run([]string{"upgrade"}, "")

		require.NoError(t, err, out)
		assert.Contains(t, out, "already runs beatstash v1.0.0")
	})

	t.Run("older release is refused", func(t *testing.T) {
		s := newSetup(t)
		s.install("")
		s.Version = "0.9.0"

		out, err := s.Run([]string{"upgrade"}, "")

		require.ErrorContains(t, err, "downgrades are not supported", out)
		assert.Contains(t, s.Read(".env"), `BEATSTASH_VERSION="1.0.0"`)
	})
}

// dump is the SQL of the only backup in dir.
func dump(t *testing.T, dir string) string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "beatstash-1.0.0-*.sql.gz"))
	require.NoError(t, err)
	require.Len(t, files, 1)
	file, err := os.Open(files[0])
	require.NoError(t, err)
	defer file.Close()
	archive, err := gzip.NewReader(file)
	require.NoError(t, err)
	sql, err := io.ReadAll(archive)
	require.NoError(t, err)
	return string(sql)
}
