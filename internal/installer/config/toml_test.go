package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/beatstash/internal/installer/config"
)

const current = `# Admins.
admins = ["telegram:1"]

[navidrome]
# Where it is.
url = "http://navidrome:4533"
user = "me"

[old]
gone = 1
`

func TestSet(t *testing.T) {
	t.Parallel()

	t.Run("value changes in its table only", func(t *testing.T) {
		got, err := config.Set(current, "navidrome", "user", config.String(`a "quoted" \ name`))

		require.NoError(t, err)
		assert.Equal(t, `# Admins.
admins = ["telegram:1"]

[navidrome]
# Where it is.
url = "http://navidrome:4533"
user = "a \"quoted\" \\ name"

[old]
gone = 1
`, got)
	})

	t.Run("top-level key changes", func(t *testing.T) {
		got, err := config.Set(current, "", "admins", `["telegram:2"]`)

		require.NoError(t, err)
		assert.Contains(t, got, "admins = [\"telegram:2\"]\n\n[navidrome]")
	})

	t.Run("missing key is an error", func(t *testing.T) {
		_, err := config.Set(current, "navidrome", "missing", `""`)

		assert.EqualError(t, err, "config.toml must have exactly one navidrome.missing")
	})
}

func TestMerge(t *testing.T) {
	t.Parallel()

	example := `# Admins.
admins = []
# Shown to strangers.
admin_contact = ""

[navidrome]
# Where it is.
url = "http://localhost:4533"
# How long a link works.
listen_link_ttl = "720h"
user = "admin"

[quota]
# Default quota.
default = ""
`

	got, err := config.Merge(current, example)

	require.NoError(t, err)
	assert.Equal(t, `# Admins.
admins = ["telegram:1"]
# Shown to strangers.
admin_contact = ""

[navidrome]
# Where it is.
url = "http://navidrome:4533"
user = "me"
# How long a link works.
listen_link_ttl = "720h"

[old]
gone = 1

[quota]
# Default quota.
default = ""
`, got.Text)
	assert.Equal(t, []string{"old.gone"}, got.Unknown)
}
