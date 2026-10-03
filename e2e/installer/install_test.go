package installer_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstall(t *testing.T) {
	t.Run("new server runs the stack with its Navidrome administrator", func(t *testing.T) {
		s := newSetup(t)

		out := s.install("")

		assert.Contains(t, out, "Installing v1.0.0 into "+s.Deploy(""))
		assert.Contains(t, out, "Open on your computer or phone: https://t.me/BotFather")
		assert.ElementsMatch(t, []string{"postgres", "telegram-bot-api", "bot", "navidrome"}, s.Running())
		assert.Equal(t, http.StatusOK, s.Login("admin", password))
		settings := s.Settings()
		assert.Equal(t, []any{"telegram:42"}, settings["admins"])
		assert.Equal(t, "@me", settings["admin_contact"])
		assert.Equal(t, map[string]any{"user": "admin", "url": "http://navidrome:4533", "public_url": ""},
			pick(settings["navidrome"], "user", "url", "public_url"))
		assert.Equal(t, []string{"POST /bot" + token + "/logOut"}, s.Telegram.Calls())
	})

	t.Run("rerun keeps every answer", func(t *testing.T) {
		s := newSetup(t)
		s.install("")
		before := s.Read("config.toml")
		started := startedAt(t, "beatstash-bot-1")

		out, err := s.Run(nil, "~/beatstash", "", "", "", "", "", "", "", "", "", "", "", "y")

		require.NoError(t, err, out)
		assert.Equal(t, before, s.Read("config.toml"))
		assert.NotEqual(t, started, startedAt(t, "beatstash-bot-1"), "the bot rereads its settings")
		assert.Contains(t, out, "Navidrome already has the administrator admin")
		assert.Len(t, s.Telegram.Calls(), 1, "the bot leaves the cloud API once")
	})

	t.Run("rejected Navidrome password is asked again", func(t *testing.T) {
		s := newSetup(t)
		s.install("")

		out, err := s.Run(nil, "~/beatstash", "", "", "", "", "", "", "", "", "", "wrong", "", "", password, "y")

		require.NoError(t, err, out)
		assert.Contains(t, out, "rejected admin with this password")
		assert.Contains(t, s.Read(".env"), `NAVIDROME_PASSWORD="`+password+`"`)
	})

	t.Run("password reaches the containers verbatim", func(t *testing.T) {
		s := newSetup(t)
		tricky := `ends\ 'quote" $HOME ${HOME} $$ # x\`

		out, err := s.Run(nil, "~/beatstash", "existing", token, "", "42", "", "1", apiHash, "",
			"https://music.example.com", "", "", "", tricky, "y", "y")

		require.NoError(t, err, out)
		assert.Equal(t, tricky+"\n", docker(t, "exec", "beatstash-bot-1", "printenv", "NAVIDROME_PASSWORD"))
	})

	t.Run("blocked Telegram goes through the given proxy", func(t *testing.T) {
		s := newSetup(t)
		s.TelegramDC = "127.0.0.1:1"
		proxy := newProxy(t)
		address := "http://host.docker.internal:" + proxy.Port

		out, err := s.Run(nil, "~/beatstash", "existing", token, "", "42", "", "1", apiHash,
			"http://127.0.0.1:"+proxy.Port, address,
			"", "https://music.example.com", "", "", "", password, "y", "y")

		require.NoError(t, err, out)
		assert.Contains(t, out, "containers cannot reach 127.0.0.1")
		assert.Contains(t, out, "Telegram answers through "+address)
		assert.Contains(t, s.Read(".env"), `TELEGRAM_PROXY="`+address+`"`)
		assert.Contains(t, proxy.Paths(), "/bot"+token+"/logOut")
		assert.Contains(t, s.Running(), "telegram-proxy")
		assert.Contains(t, docker(t, "inspect", "-f", "{{.HostConfig.NetworkMode}}", "beatstash-telegram-bot-api-1"), "container:")
	})

	t.Run("existing Navidrome is connected without starting another", func(t *testing.T) {
		s := newSetup(t)
		s.Telegram.Answer = `{"ok":false,"error_code":400,"description":"Bad Request: Logged out"}`

		out, err := s.Run(nil, "~/beatstash", "existing", token, "", "42", "", "1", apiHash, "",
			"http://localhost:4533", "https://music.example.com", "", "", "", password, "y", "y")

		require.NoError(t, err, out)
		assert.Contains(t, out, "localhost inside the bot points to the bot itself")
		assert.Contains(t, out, "- "+s.Deploy("music")+":/beatstash-music:ro")
		assert.Contains(t, out, "Skipped: your existing Navidrome keeps its own proxy")
		assert.Equal(t, map[string]any{"url": "https://music.example.com"}, pick(s.Settings()["navidrome"], "url"))
		assert.Equal(t, map[string]any{"navidrome_music_dir": "/beatstash-music"}, pick(s.Settings()["library"], "navidrome_music_dir"))
		assert.ElementsMatch(t, []string{"postgres", "telegram-bot-api", "bot"}, s.Running())
	})
}

func startedAt(t *testing.T, container string) string {
	t.Helper()
	return docker(t, "inspect", "-f", "{{.State.StartedAt}}", container)
}

func pick(table any, keys ...string) map[string]any {
	values := map[string]any{}
	for _, key := range keys {
		values[key] = table.(map[string]any)[key]
	}
	return values
}
