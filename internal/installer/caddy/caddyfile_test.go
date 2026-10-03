package caddy_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/beatstash/internal/installer/caddy"
)

const sites = `solar.example.com {
	reverse_proxy 127.0.0.1:8080
}
`

func TestCaddyfile(t *testing.T) {
	t.Parallel()

	t.Run("site is appended after the others", func(t *testing.T) {
		got, err := caddy.WithSite(sites, "music.example.com", "127.0.0.1:4533")

		require.NoError(t, err)
		assert.Equal(t, `solar.example.com {
	reverse_proxy 127.0.0.1:8080
}

# beatstash:begin
music.example.com {
	reverse_proxy 127.0.0.1:4533
}
# beatstash:end
`, got)
	})

	t.Run("site replaces the previous one", func(t *testing.T) {
		old, err := caddy.WithSite(sites, "old.example.com", "127.0.0.1:4533")
		require.NoError(t, err)

		got, err := caddy.WithSite(old, "music.example.com", "127.0.0.1:4533")

		require.NoError(t, err)
		want, err := caddy.WithSite(sites, "music.example.com", "127.0.0.1:4533")
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("site added later stays after the block", func(t *testing.T) {
		with, err := caddy.WithSite(sites, "old.example.com", "127.0.0.1:4533")
		require.NoError(t, err)
		later := with + "\nlater.example.com {\n\trespond ok\n}\n"

		got, err := caddy.WithSite(later, "music.example.com", "127.0.0.1:4533")

		require.NoError(t, err)
		assert.Equal(t, strings.Replace(later, "old.example.com", "music.example.com", 1), got)
	})

	t.Run("file without final newline keeps its sites", func(t *testing.T) {
		got, err := caddy.WithSite("solar.example.com {\n\trespond ok\n}", "music.example.com", "127.0.0.1:4533")

		require.NoError(t, err)
		assert.Equal(t, "solar.example.com {\n\trespond ok\n}\n\n# beatstash:begin\nmusic.example.com {\n\treverse_proxy 127.0.0.1:4533\n}\n# beatstash:end\n", got)
	})

	t.Run("removing restores the file", func(t *testing.T) {
		with, err := caddy.WithSite(sites, "music.example.com", "127.0.0.1:4533")
		require.NoError(t, err)

		assert.Equal(t, sites, caddy.WithoutSite(with))
	})

	t.Run("domain served outside the block is refused", func(t *testing.T) {
		for _, address := range []string{"music.example.com", "https://music.example.com", "www.example.com, music.example.com:443"} {
			_, err := caddy.WithSite(address+" {\n\trespond ok\n}\n", "music.example.com", "127.0.0.1:4533")

			assert.ErrorIs(t, err, caddy.ErrForeignSite, address)
		}
	})

	t.Run("similar domain is not a conflict", func(t *testing.T) {
		_, err := caddy.WithSite("old-music.example.com {\n\trespond ok\n}\n# music.example.com later\n", "music.example.com", "127.0.0.1:4533")

		assert.NoError(t, err)
	})
}
