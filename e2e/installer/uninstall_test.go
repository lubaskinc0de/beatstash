package installer_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUninstall(t *testing.T) {
	t.Parallel()
	t.Run("defaults stop the stack, take the site out of Caddy and keep data", func(t *testing.T) {
		s := newSetup(t)
		caddyfile := s.startCaddy(s.CaddyGlobals())
		s.install("https://"+domain, "y", "y", "y")

		out, err := s.Run([]string{"uninstall"}, "", "", "", "", "y", "y", "", "", "")

		require.NoError(t, err, out)
		assert.Equal(t, s.CaddyGlobals(), fileText(t, caddyfile))
		assert.Empty(t, s.Running())
		assert.Len(t, s.Volumes(), 2)
		assert.FileExists(t, s.Deploy("config.toml"))
	})

	t.Run("agreeing to everything removes every trace", func(t *testing.T) {
		s := newSetup(t)
		s.install("")

		out, err := s.Run([]string{"uninstall"}, "", "y", "", "y", "y", "y", "y")

		require.NoError(t, err, out)
		assert.Contains(t, out, "The bot left the local Telegram API.")
		assert.Contains(t, out, "beatstash is fully removed.")
		assert.Empty(t, s.Running())
		assert.Empty(t, s.Volumes())
		assert.NoDirExists(t, filepath.Join(s.Home, "beatstash"))
	})
}

func (s *Setup) Volumes() []string {
	t := s.t
	t.Helper()
	return strings.Fields(docker(t, "volume", "ls", "-q", "--filter", "label=com.docker.compose.project="+s.ProjectName))
}
