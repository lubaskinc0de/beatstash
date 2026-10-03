package beatstash

import _ "embed"

// Templates of a deployment, built into the setup tool so it always installs
// the files of its own release.
var (
	//go:embed deploy/compose.yml
	ComposeTemplate string
	//go:embed config.example.toml
	ConfigTemplate string
	//go:embed deploy/compose.telegram-proxy.yml
	TelegramProxyTemplate string
)
