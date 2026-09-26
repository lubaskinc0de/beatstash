package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/navidrome"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/telegram"
)

func TestNavidromeLibraryGone(t *testing.T) {
	t.Parallel()

	t.Run("library deleted in Navidrome is found again by its path", func(t *testing.T) {
		s := harness.New(t)
		account := s.LinkNewAccount(alice)
		s.Navidrome.DeleteLibrary(t, s.Navidrome.LibraryAt(t, s.NavidromePath("shared")))
		id := s.Navidrome.CreateLibrary(t, navidrome.UniqueLogin("moved"), s.NavidromePath("shared"))

		s.Restart()

		assert.Equal(t, []string{
			s.NavidromePath("shared"),
			s.NavidromePath(s.PersonalDir(alice)),
		}, s.Navidrome.Libraries(t, account))
		assert.Equal(t, id, s.Navidrome.LibraryAt(t, s.NavidromePath("shared")))
	})

	t.Run("library deleted in Navidrome is created again", func(t *testing.T) {
		s := harness.New(t)
		account := s.LinkNewAccount(alice)
		s.Navidrome.DeleteLibrary(t, s.Navidrome.LibraryAt(t, s.NavidromePath("shared")))

		s.Restart()

		assert.Equal(t, []string{
			s.NavidromePath("shared"),
			s.NavidromePath(s.PersonalDir(alice)),
		}, s.Navidrome.Libraries(t, account))
	})
}

func TestStartup(t *testing.T) {
	t.Parallel()

	t.Run("bot refuses to start without SECRET_KEY", func(t *testing.T) {
		binary := buildBot(t)
		vars := botEnv(t)
		delete(vars, "SECRET_KEY")

		out, err := runBot(t, binary, vars)

		var exit *exec.ExitError
		require.ErrorAs(t, err, &exit)
		assert.NotZero(t, exit.ExitCode())
		assert.Contains(t, out, "SECRET_KEY")
	})

	t.Run("bot refuses to start with a malformed SECRET_KEY", func(t *testing.T) {
		binary := buildBot(t)
		vars := botEnv(t)
		vars["SECRET_KEY"] = "too-short"

		out, err := runBot(t, binary, vars)

		var exit *exec.ExitError
		require.ErrorAs(t, err, &exit)
		assert.NotZero(t, exit.ExitCode())
		assert.Contains(t, out, "SECRET_KEY")
	})

	t.Run("bot names every missing secret at once", func(t *testing.T) {
		binary := buildBot(t)
		vars := botEnv(t)
		delete(vars, "BOT_TOKEN")
		delete(vars, "SECRET_KEY")

		out, err := runBot(t, binary, vars)

		var exit *exec.ExitError
		require.ErrorAs(t, err, &exit)
		assert.Contains(t, out, "BOT_TOKEN")
		assert.Contains(t, out, "SECRET_KEY")
	})

	t.Run("bot refuses an unknown setting", func(t *testing.T) {
		binary := buildBot(t)
		vars := botEnv(t)
		vars["CONFIG_FILE"] = writeConfig(t, "admin_idz = [1000]\n"+botConfig(t))

		out, err := runBot(t, binary, vars)

		var exit *exec.ExitError
		require.ErrorAs(t, err, &exit)
		assert.Contains(t, out, "unknown key admin_idz")
	})

	t.Run("bot refuses a zero poll interval", func(t *testing.T) {
		binary := buildBot(t)
		vars := botEnv(t)
		vars["CONFIG_FILE"] = writeConfig(t, botConfig(t)+"\n[ingest]\npoll_interval = \"0s\"\n")

		out, err := runBot(t, binary, vars)

		var exit *exec.ExitError
		require.ErrorAs(t, err, &exit)
		assert.Contains(t, out, "ingest.poll_interval")
	})

	t.Run("bot without a config file still names missing secrets", func(t *testing.T) {
		binary := buildBot(t)
		vars := botEnv(t)
		vars["CONFIG_FILE"] = filepath.Join(t.TempDir(), "missing.toml")
		delete(vars, "SECRET_KEY")

		out, err := runBot(t, binary, vars)

		var exit *exec.ExitError
		require.ErrorAs(t, err, &exit)
		assert.Contains(t, out, "missing.toml")
		assert.Contains(t, out, "SECRET_KEY")
	})
}

func buildBot(t *testing.T) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "navidrome-tg")
	out, err := exec.CommandContext(t.Context(), "go", "build", "-o", binary, "../cmd/navidrome-tg").CombinedOutput() //nolint:gosec // G204: builds the app under test
	require.NoError(t, err, string(out))
	return binary
}

func botEnv(t *testing.T) map[string]string {
	return map[string]string{
		"CONFIG_FILE":        writeConfig(t, botConfig(t)),
		"BOT_TOKEN":          telegram.Token,
		"DB_DSN":             harness.NewDatabase(t),
		"SECRET_KEY":         harness.SecretKey,
		"NAVIDROME_PASSWORD": navidrome.AdminPassword,
	}
}

func botConfig(t *testing.T) string {
	return fmt.Sprintf(`admins = ["telegram:1000"]

[telegram]
bot_api_url = %q

[library]
music_dir = %q

[navidrome]
url = %q
user = %q
`, telegram.New(t).URL(), t.TempDir(), harness.Navidrome().URL, navidrome.AdminUser)
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644)) //nolint:gosec // G306: test config, no secrets
	return path
}

func runBot(t *testing.T, binary string, vars map[string]string) (string, error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH")}
	for key, value := range vars {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	out, err := cmd.CombinedOutput()
	require.NoError(t, ctx.Err(), "bot kept running")
	return string(out), err
}
