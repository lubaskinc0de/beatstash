package installer_test

import (
	"net/http"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPS(t *testing.T) {
	t.Parallel()
	t.Run("shared Caddy serves Navidrome beside its other sites", func(t *testing.T) {
		s := newSetup(t)
		caddyfile := s.startCaddy(s.CaddyGlobals())
		inode := inodeOf(t, caddyfile)

		out := s.install("https://"+domain, "y", "y", "y")

		assert.Contains(t, out, "Navidrome answers at https://"+domain)
		written := fileText(t, caddyfile)
		assert.True(t, strings.HasPrefix(written, s.CaddyGlobals()), written)
		assert.Equal(t, inode, inodeOf(t, caddyfile), "a single-file mount sees only the original inode")
		assert.Equal(t, s.CaddyGlobals(), s.Read("Caddyfile.backup"))
		status, body := s.Get("https://other.example.test/")
		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, "other site", body)
	})

	t.Run("rerun finds the site in place", func(t *testing.T) {
		s := newSetup(t)
		s.startCaddy(s.CaddyGlobals())
		s.install("https://"+domain, "y", "y", "y")

		out, err := s.Run(nil, "~/beatstash", "", "", "", "", "", "", "", "", "", "", "", "y")

		require.NoError(t, err, out)
		assert.Contains(t, out, "Caddy already serves "+domain)
	})

	t.Run("site Caddy serves already is left alone", func(t *testing.T) {
		s := newSetup(t)
		own := s.CaddyGlobals() + "\n" + domain + " {\n\trespond \"mine\"\n}\n"
		caddyfile := s.startCaddy(own)

		out := s.install("https://" + domain)

		assert.Contains(t, out, "already serves "+domain+" outside the beatstash block")
		assert.Contains(t, out, "reverse_proxy 127.0.0.1:")
		assert.Equal(t, own, fileText(t, caddyfile))
	})

	t.Run("Caddy changed past its file is left alone", func(t *testing.T) {
		s := newSetup(t)
		caddyfile := s.startCaddy(s.CaddyGlobals())
		docker(t, "exec", s.CaddyName, "sh", "-c",
			`sed 's/other site/changed/' /etc/caddy/Caddyfile > /tmp/Caddyfile && caddy reload --config /tmp/Caddyfile --adapter caddyfile`)

		out := s.install("https://"+domain, "y")

		assert.Contains(t, out, "running Caddy config differs from its Caddyfile")
		assert.Equal(t, s.CaddyGlobals(), fileText(t, caddyfile))
	})

	t.Run("declined change leaves Caddy as it was", func(t *testing.T) {
		s := newSetup(t)
		caddyfile := s.startCaddy(s.CaddyGlobals())

		out := s.install("https://"+domain, "y", "n")

		assert.Contains(t, out, "+ "+domain+" {")
		assert.Equal(t, s.CaddyGlobals(), fileText(t, caddyfile))
	})

	t.Run("without a proxy the stack runs its own Caddy", func(t *testing.T) {
		s := newSetup(t)

		out := s.install("https://"+domain, "y", "admin@example.test", "n")

		assert.Contains(t, out, "Start Caddy for "+domain)
		assert.Contains(t, s.Read(".env"), `COMPOSE_PROFILES="navidrome,proxy"`)
		assert.Contains(t, s.Read("Caddyfile"), "email admin@example.test")
		assert.Contains(t, s.Read("Caddyfile"), domain+" {\n\treverse_proxy navidrome:4533\n}")
		assert.Contains(t, s.Running(), "caddy")
	})
}

func inodeOf(t *testing.T, path string) uint64 {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err)
	return info.Sys().(*syscall.Stat_t).Ino
}
