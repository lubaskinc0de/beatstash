package app

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

const defaultConfigFile = "config.toml"

type Config struct {
	Token     string
	BotAPIURL string
	DBDSN     string
	MusicDir  string
	// NavidromeMusicDir is MusicDir as Navidrome's container sees it.
	NavidromeMusicDir string
	AdminIDs          []uint64
	AdminContact      string
	// SecretKey is a base64-encoded 32-byte AES key for stored secrets.
	SecretKey         string
	NavidromeUser     string
	NavidromePassword string
	NavidromeURL      string

	InviteTTL time.Duration
	Clock     func() time.Time

	IngestWorkers int
	// IngestRetryDelays are waits before each retry of a failed Ingest Job.
	IngestRetryDelays  []time.Duration
	IngestPollInterval time.Duration
}

type fileConfig struct {
	AdminIDs     []uint64 `toml:"admin_ids"`
	AdminContact string   `toml:"admin_contact"`

	Telegram struct {
		BotAPIURL string `toml:"bot_api_url"`
	} `toml:"telegram"`

	Library struct {
		MusicDir          string `toml:"music_dir"`
		NavidromeMusicDir string `toml:"navidrome_music_dir"`
	} `toml:"library"`

	Navidrome struct {
		URL  string `toml:"url"`
		User string `toml:"user"`
	} `toml:"navidrome"`

	Invites struct {
		TTL time.Duration `toml:"ttl"`
	} `toml:"invites"`

	Ingest struct {
		Workers      int             `toml:"workers"`
		RetryDelays  []time.Duration `toml:"retry_delays"`
		PollInterval time.Duration   `toml:"poll_interval"`
	} `toml:"ingest"`
}

func defaultFileConfig() fileConfig {
	var f fileConfig
	f.Invites.TTL = 7 * 24 * time.Hour
	f.Ingest.Workers = 2
	f.Ingest.RetryDelays = []time.Duration{10 * time.Second, time.Minute, 5 * time.Minute}
	f.Ingest.PollInterval = time.Second
	return f
}

// LoadConfig reads settings from the TOML file named by CONFIG_FILE and
// secrets from the environment, reporting every problem at once.
func LoadConfig() (Config, error) {
	path := os.Getenv("CONFIG_FILE")
	if path == "" {
		path = defaultConfigFile
	}

	file := defaultFileConfig()
	var problems []error
	for _, err := range readFile(path, &file) {
		problems = append(problems, fmt.Errorf("config file %s: %w", path, err))
	}

	secret := func(name string) string {
		value := os.Getenv(name)
		if value == "" {
			problems = append(problems, fmt.Errorf("environment variable %s is not set", name))
		}
		return value
	}

	navidromeMusicDir := file.Library.NavidromeMusicDir
	if navidromeMusicDir == "" {
		navidromeMusicDir = file.Library.MusicDir
	}

	cfg := Config{
		Token:             secret("BOT_TOKEN"),
		BotAPIURL:         file.Telegram.BotAPIURL,
		DBDSN:             secret("DB_DSN"),
		MusicDir:          file.Library.MusicDir,
		NavidromeMusicDir: navidromeMusicDir,
		AdminIDs:          file.AdminIDs,
		AdminContact:      strings.TrimSpace(file.AdminContact),
		SecretKey:         secret("SECRET_KEY"),
		NavidromeUser:     file.Navidrome.User,
		NavidromePassword: secret("NAVIDROME_PASSWORD"),
		NavidromeURL:      file.Navidrome.URL,

		InviteTTL: file.Invites.TTL,
		Clock:     time.Now,

		IngestWorkers:      file.Ingest.Workers,
		IngestRetryDelays:  file.Ingest.RetryDelays,
		IngestPollInterval: file.Ingest.PollInterval,
	}
	return cfg, errors.Join(problems...)
}

func readFile(path string, file *fileConfig) []error {
	meta, err := toml.DecodeFile(path, file)
	if err != nil {
		return []error{err}
	}

	var problems []error
	for _, key := range meta.Undecoded() {
		problems = append(problems, fmt.Errorf("unknown key %s", key))
	}
	require := func(missing bool, key string) {
		if missing {
			problems = append(problems, fmt.Errorf("%s is required", key))
		}
	}
	require(len(file.AdminIDs) == 0, "admin_ids")
	require(file.Library.MusicDir == "", "library.music_dir")
	require(file.Navidrome.URL == "", "navidrome.url")
	require(file.Navidrome.User == "", "navidrome.user")
	if file.Ingest.Workers < 1 {
		problems = append(problems, errors.New("ingest.workers must be at least 1"))
	}
	if file.Ingest.PollInterval <= 0 {
		problems = append(problems, errors.New("ingest.poll_interval must be positive"))
	}
	return problems
}
