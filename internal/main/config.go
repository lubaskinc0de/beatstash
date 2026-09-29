package app

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
)

const defaultConfigFile = "config.toml"

type Config struct {
	Token       string
	BotAPIURL   string
	MaxPostSize int64
	// StorageChatID is where the bot posts Tracks that came without a
	// Telegram file; zero posts them on first request only.
	StorageChatID int64
	// TelegramPollInterval is how often the bot looks for answers it owes.
	TelegramPollInterval time.Duration
	DBDSN                string
	MusicDir             string
	// NavidromeMusicDir is MusicDir as Navidrome's container sees it.
	NavidromeMusicDir string
	// Admins are Identities such as telegram:123; StartApp makes them Admins.
	Admins       []access.Identity
	AdminContact string
	ServiceName  string
	// TranslationsDir adds or overrides language files; empty: built-in only.
	TranslationsDir string
	// DefaultLanguage is for clients whose language has no file.
	DefaultLanguage string
	// SecretKey is a base64-encoded 32-byte AES key for stored secrets.
	SecretKey         string
	NavidromeUser     string
	NavidromePassword string
	NavidromeURL      string
	// NavidromePublicURL is where users open Navidrome; empty hides it.
	NavidromePublicURL string
	// AttachInterval is how often the bot looks for songs of Attached
	// Libraries; zero takes no Attached Libraries at all.
	AttachInterval time.Duration

	InviteTTL time.Duration
	Clock     func() time.Time

	IngestWorkers int
	// IngestRetryDelays are waits before each retry of a failed Ingest Job.
	IngestRetryDelays  []time.Duration
	IngestPollInterval time.Duration

	ZvukURL string
	// ZvukWorkers is how many Zvuk downloads run at once for all users;
	// ZvukPerUser caps those of one user.
	ZvukWorkers  int
	ZvukPerUser  int
	ZvukPauseMin time.Duration
	ZvukPauseMax time.Duration
	SyncInterval time.Duration
	// MirrorRetryInterval: stars and playlists need songs Navidrome has
	// indexed; until it has, the bot tries again this often.
	MirrorRetryInterval time.Duration
}

type fileConfig struct {
	Admins       []string `toml:"admins"`
	AdminContact string   `toml:"admin_contact"`
	ServiceName  string   `toml:"service_name"`

	I18n struct {
		Dir             string `toml:"dir"`
		DefaultLanguage string `toml:"default_language"`
	} `toml:"i18n"`

	Telegram struct {
		BotAPIURL     string        `toml:"bot_api_url"`
		StorageChatID int64         `toml:"storage_chat_id"`
		PollInterval  time.Duration `toml:"poll_interval"`
	} `toml:"telegram"`

	Library struct {
		MusicDir          string `toml:"music_dir"`
		NavidromeMusicDir string `toml:"navidrome_music_dir"`
	} `toml:"library"`

	Navidrome struct {
		URL            string        `toml:"url"`
		PublicURL      string        `toml:"public_url"`
		User           string        `toml:"user"`
		AttachInterval time.Duration `toml:"attach_interval"`
	} `toml:"navidrome"`

	Invites struct {
		TTL time.Duration `toml:"ttl"`
	} `toml:"invites"`

	Ingest struct {
		Workers      int             `toml:"workers"`
		RetryDelays  []time.Duration `toml:"retry_delays"`
		PollInterval time.Duration   `toml:"poll_interval"`
	} `toml:"ingest"`

	Zvuk struct {
		URL                 string          `toml:"url"`
		Workers             int             `toml:"workers"`
		PerUser             int             `toml:"per_user"`
		Pause               []time.Duration `toml:"pause"`
		SyncInterval        time.Duration   `toml:"sync_interval"`
		MirrorRetryInterval time.Duration   `toml:"mirror_retry_interval"`
	} `toml:"zvuk"`
}

func defaultFileConfig() fileConfig {
	var f fileConfig
	f.ServiceName = "navidrome-tg"
	f.I18n.DefaultLanguage = "en"
	f.Telegram.PollInterval = 2 * time.Second
	f.Navidrome.AttachInterval = time.Hour
	f.Invites.TTL = 7 * 24 * time.Hour
	f.Ingest.Workers = 2
	f.Ingest.RetryDelays = []time.Duration{10 * time.Second, time.Minute, 5 * time.Minute}
	f.Ingest.PollInterval = time.Second
	f.Zvuk.URL = "https://zvuk.com"
	f.Zvuk.Workers = 4
	f.Zvuk.PerUser = 1
	f.Zvuk.Pause = []time.Duration{5 * time.Second, 10 * time.Second}
	f.Zvuk.SyncInterval = 6 * time.Hour
	f.Zvuk.MirrorRetryInterval = time.Minute
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

	var pauseMin, pauseMax time.Duration //nolint:revive // Min is minimum, not minutes
	if len(file.Zvuk.Pause) == 2 {
		pauseMin, pauseMax = file.Zvuk.Pause[0], file.Zvuk.Pause[1]
	}

	cfg := Config{
		Token:                secret("BOT_TOKEN"),
		BotAPIURL:            file.Telegram.BotAPIURL,
		MaxPostSize:          maxPostSize(file.Telegram.BotAPIURL),
		StorageChatID:        file.Telegram.StorageChatID,
		TelegramPollInterval: file.Telegram.PollInterval,
		DBDSN:                secret("DB_DSN"),
		MusicDir:             file.Library.MusicDir,
		NavidromeMusicDir:    navidromeMusicDir,
		Admins:               parseIdentities(file.Admins, &problems),
		AdminContact:         strings.TrimSpace(file.AdminContact),
		ServiceName:          strings.TrimSpace(file.ServiceName),
		TranslationsDir:      file.I18n.Dir,
		DefaultLanguage:      file.I18n.DefaultLanguage,
		SecretKey:            secret("SECRET_KEY"),
		NavidromeUser:        file.Navidrome.User,
		NavidromePassword:    secret("NAVIDROME_PASSWORD"),
		NavidromeURL:         file.Navidrome.URL,
		NavidromePublicURL:   strings.TrimSpace(file.Navidrome.PublicURL),
		AttachInterval:       file.Navidrome.AttachInterval,

		InviteTTL: file.Invites.TTL,
		Clock:     time.Now,

		IngestWorkers:      file.Ingest.Workers,
		IngestRetryDelays:  file.Ingest.RetryDelays,
		IngestPollInterval: file.Ingest.PollInterval,

		ZvukURL:             file.Zvuk.URL,
		ZvukWorkers:         file.Zvuk.Workers,
		ZvukPerUser:         file.Zvuk.PerUser,
		ZvukPauseMin:        pauseMin,
		ZvukPauseMax:        pauseMax,
		SyncInterval:        file.Zvuk.SyncInterval,
		MirrorRetryInterval: file.Zvuk.MirrorRetryInterval,
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
	require(len(file.Admins) == 0, "admins")
	require(file.Library.MusicDir == "", "library.music_dir")
	require(file.Navidrome.URL == "", "navidrome.url")
	require(file.Navidrome.User == "", "navidrome.user")
	if file.Navidrome.AttachInterval < 0 {
		problems = append(problems, errors.New("navidrome.attach_interval must not be negative"))
	}
	if file.Telegram.PollInterval <= 0 {
		problems = append(problems, errors.New("telegram.poll_interval must be positive"))
	}
	if file.Ingest.Workers < 1 {
		problems = append(problems, errors.New("ingest.workers must be at least 1"))
	}
	if file.Ingest.PollInterval <= 0 {
		problems = append(problems, errors.New("ingest.poll_interval must be positive"))
	}
	require(file.Zvuk.URL == "", "zvuk.url")
	if file.Zvuk.Workers < 1 {
		problems = append(problems, errors.New("zvuk.workers must be at least 1"))
	}
	if file.Zvuk.PerUser < 1 {
		problems = append(problems, errors.New("zvuk.per_user must be at least 1"))
	}
	if len(file.Zvuk.Pause) != 2 || file.Zvuk.Pause[0] < 0 || file.Zvuk.Pause[0] > file.Zvuk.Pause[1] {
		problems = append(problems, errors.New(`zvuk.pause must be a range like ["5s", "10s"]`))
	}
	if file.Zvuk.SyncInterval <= 0 {
		problems = append(problems, errors.New("zvuk.sync_interval must be positive"))
	}
	if file.Zvuk.MirrorRetryInterval <= 0 {
		problems = append(problems, errors.New("zvuk.mirror_retry_interval must be positive"))
	}
	return problems
}

// maxPostSize is 50 MB on Telegram's Bot API; a local one takes up to 2 GB.
func maxPostSize(botAPIURL string) int64 {
	if botAPIURL == "" {
		return 50 << 20
	}
	return 2000 << 20
}

// parseIdentities reads "channel:external_id" pairs.
func parseIdentities(values []string, problems *[]error) []access.Identity {
	identities := make([]access.Identity, 0, len(values))
	for _, value := range values {
		channel, id, ok := strings.Cut(value, ":")
		if !ok || channel == "" || id == "" {
			*problems = append(*problems, fmt.Errorf(`admins: %q is not like "telegram:123"`, value))
			continue
		}
		identities = append(identities, access.Identity{Channel: access.Channel(channel), ExternalID: id})
	}
	return identities
}
