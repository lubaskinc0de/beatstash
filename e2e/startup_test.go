package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartup(t *testing.T) {
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
}

func buildBot(t *testing.T) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "navidrome-tg")
	out, err := exec.Command("go", "build", "-o", binary, "../cmd/navidrome-tg").CombinedOutput()
	require.NoError(t, err, string(out))
	return binary
}

func botEnv(t *testing.T) map[string]string {
	return map[string]string{
		"BOT_TOKEN":          botToken,
		"BOT_API_URL":        newBotAPI(t).URL(),
		"DB_DSN":             postgresDSN(createDatabase(t)),
		"MUSIC_DIR":          t.TempDir(),
		"ADMIN_IDS":          "1000",
		"SECRET_KEY":         secretKey,
		"NAVIDROME_URL":      env.navidrome.url,
		"NAVIDROME_USER":     navidromeAdmin,
		"NAVIDROME_PASSWORD": navidromePassword,
	}
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
