package app

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/lubaskinc0de/beatstash/internal/domain/access"
	"github.com/lubaskinc0de/beatstash/internal/domain/library"
	"github.com/lubaskinc0de/beatstash/internal/infra/stall"
)

const defaultConfigFile = "config.toml"

type Config struct {
	Token       string
	BotAPIURL   string
	MaxPostSize int64
	// StorageChatID is where the bot posts Tracks that came without a
	// Telegram file; zero posts them on first request only.
	StorageChatID int64
	// FillStorageChat uploads every Track without a Telegram file to the
	// storage chat in the background; off, only the ones users choose.
	FillStorageChat bool
	// TelegramPollInterval is how often the bot looks for answers it owes.
	TelegramPollInterval time.Duration
	// TelegramLeaseTTL bounds how long a crashed instance keeps a chat's
	// window, an answer or an upload to itself.
	TelegramLeaseTTL time.Duration
	DBDSN            string
	MusicDir         string
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
	// ListenLinkTTL is how long a Listen Link lasts; ListenLinkDownloadable
	// lets its listener download the files too.
	ListenLinkTTL          time.Duration
	ListenLinkDownloadable bool
	// SongInterval is how often the bot learns which Navidrome songs its
	// new Tracks became; zero leaves it to Listen Links made on demand.
	SongInterval time.Duration
	// AttachInterval is how often the bot looks for songs of Attached
	// Libraries; zero takes no Attached Libraries at all.
	AttachInterval time.Duration
	// NavidromeAccessTTL is how long the bot trusts what Navidrome said an
	// account sees; zero asks Navidrome on every request.
	NavidromeAccessTTL time.Duration
	// Quotas hold until the Admin sets others in the bot.
	Quotas library.ServerQuotas

	InviteTTL time.Duration
	// Clock and AfterFunc are the bot's time.
	Clock     func() time.Time
	AfterFunc func(d time.Duration, f func()) stall.Timer

	IngestWorkers int
	// IngestRetryDelays are waits before each retry of a failed Ingest Job.
	IngestRetryDelays  []time.Duration
	IngestPollInterval time.Duration
	// ScratchTTL is how old a scratch file gets before the bot takes it for
	// one a crashed Ingest left behind.
	ScratchTTL time.Duration
	// StallTimeout cuts off a download that sent nothing for this long.
	StallTimeout time.Duration
	// ReconcileInterval is how often the bot brings the Tracks of the
	// Personal Libraries and the Shared Library in step with their files.
	ReconcileInterval time.Duration

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
		BotAPIURL       string        `toml:"bot_api_url"`
		StorageChatID   int64         `toml:"storage_chat_id"`
		FillStorageChat bool          `toml:"fill_storage_chat"`
		PollInterval    time.Duration `toml:"poll_interval"`
		LeaseTTL        time.Duration `toml:"lease_ttl"`
	} `toml:"telegram"`

	Library struct {
		MusicDir          string        `toml:"music_dir"`
		NavidromeMusicDir string        `toml:"navidrome_music_dir"`
		ReconcileInterval time.Duration `toml:"reconcile_interval"`
	} `toml:"library"`

	Navidrome struct {
		URL                    string        `toml:"url"`
		PublicURL              string        `toml:"public_url"`
		User                   string        `toml:"user"`
		AttachInterval         time.Duration `toml:"attach_interval"`
		ListenLinkTTL          time.Duration `toml:"listen_link_ttl"`
		ListenLinkDownloadable bool          `toml:"listen_link_downloadable"`
		SongInterval           time.Duration `toml:"song_interval"`
		AccessTTL              time.Duration `toml:"access_ttl"`
	} `toml:"navidrome"`

	Invites struct {
		TTL time.Duration `toml:"ttl"`
	} `toml:"invites"`

	Quota struct {
		Default string `toml:"default"`
		Shared  string `toml:"shared"`
	} `toml:"quota"`

	Ingest struct {
		Workers      int             `toml:"workers"`
		RetryDelays  []time.Duration `toml:"retry_delays"`
		PollInterval time.Duration   `toml:"poll_interval"`
		ScratchTTL   time.Duration   `toml:"scratch_ttl"`
		StallTimeout time.Duration   `toml:"stall_timeout"`
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
	f.ServiceName = "beatstash"
	f.I18n.DefaultLanguage = "en"
	f.Telegram.PollInterval = 2 * time.Second
	f.Telegram.LeaseTTL = time.Minute
	f.Telegram.FillStorageChat = true
	f.Navidrome.AttachInterval = time.Hour
	f.Navidrome.ListenLinkTTL = 720 * time.Hour
	f.Navidrome.ListenLinkDownloadable = true
	f.Navidrome.SongInterval = time.Minute
	f.Navidrome.AccessTTL = 30 * time.Second
	f.Library.ReconcileInterval = time.Hour
	f.Invites.TTL = 7 * 24 * time.Hour
	f.Ingest.Workers = 2
	f.Ingest.RetryDelays = []time.Duration{10 * time.Second, time.Minute, 5 * time.Minute}
	f.Ingest.PollInterval = time.Second
	f.Ingest.ScratchTTL = time.Hour
	f.Ingest.StallTimeout = time.Minute
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
		Token:                  secret("BOT_TOKEN"),
		BotAPIURL:              file.Telegram.BotAPIURL,
		MaxPostSize:            maxPostSize(file.Telegram.BotAPIURL),
		StorageChatID:          file.Telegram.StorageChatID,
		FillStorageChat:        file.Telegram.FillStorageChat,
		TelegramPollInterval:   file.Telegram.PollInterval,
		TelegramLeaseTTL:       file.Telegram.LeaseTTL,
		DBDSN:                  secret("DB_DSN"),
		MusicDir:               file.Library.MusicDir,
		NavidromeMusicDir:      navidromeMusicDir,
		Admins:                 parseIdentities(file.Admins, &problems),
		AdminContact:           strings.TrimSpace(file.AdminContact),
		ServiceName:            strings.TrimSpace(file.ServiceName),
		TranslationsDir:        file.I18n.Dir,
		DefaultLanguage:        file.I18n.DefaultLanguage,
		SecretKey:              secret("SECRET_KEY"),
		NavidromeUser:          file.Navidrome.User,
		NavidromePassword:      secret("NAVIDROME_PASSWORD"),
		NavidromeURL:           file.Navidrome.URL,
		NavidromePublicURL:     strings.TrimSpace(file.Navidrome.PublicURL),
		AttachInterval:         file.Navidrome.AttachInterval,
		ListenLinkTTL:          file.Navidrome.ListenLinkTTL,
		ListenLinkDownloadable: file.Navidrome.ListenLinkDownloadable,
		SongInterval:           file.Navidrome.SongInterval,
		NavidromeAccessTTL:     file.Navidrome.AccessTTL,
		Quotas: library.ServerQuotas{
			Default: parseQuota("quota.default", file.Quota.Default, &problems),
			Shared:  parseQuota("quota.shared", file.Quota.Shared, &problems),
		},

		InviteTTL: file.Invites.TTL,
		Clock:     time.Now,
		AfterFunc: stall.RealTime,

		IngestWorkers:      file.Ingest.Workers,
		IngestRetryDelays:  file.Ingest.RetryDelays,
		IngestPollInterval: file.Ingest.PollInterval,
		ScratchTTL:         file.Ingest.ScratchTTL,
		StallTimeout:       file.Ingest.StallTimeout,
		ReconcileInterval:  file.Library.ReconcileInterval,

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
	if file.Library.ReconcileInterval <= 0 {
		problems = append(problems, errors.New("library.reconcile_interval must be positive"))
	}
	require(file.Navidrome.URL == "", "navidrome.url")
	require(file.Navidrome.User == "", "navidrome.user")
	if file.Navidrome.AttachInterval < 0 {
		problems = append(problems, errors.New("navidrome.attach_interval must not be negative"))
	}
	if file.Navidrome.SongInterval < 0 {
		problems = append(problems, errors.New("navidrome.song_interval must not be negative"))
	}
	if file.Navidrome.AccessTTL < 0 {
		problems = append(problems, errors.New("navidrome.access_ttl must not be negative"))
	}
	if file.Navidrome.ListenLinkTTL <= 0 {
		problems = append(problems, errors.New("navidrome.listen_link_ttl must be positive"))
	}
	if file.Telegram.PollInterval <= 0 {
		problems = append(problems, errors.New("telegram.poll_interval must be positive"))
	}
	if file.Telegram.LeaseTTL <= 0 {
		problems = append(problems, errors.New("telegram.lease_ttl must be positive"))
	}
	if file.Ingest.Workers < 1 {
		problems = append(problems, errors.New("ingest.workers must be at least 1"))
	}
	if file.Ingest.PollInterval <= 0 {
		problems = append(problems, errors.New("ingest.poll_interval must be positive"))
	}
	if file.Ingest.ScratchTTL <= 0 {
		problems = append(problems, errors.New("ingest.scratch_ttl must be positive"))
	}
	if file.Ingest.StallTimeout <= 0 {
		problems = append(problems, errors.New("ingest.stall_timeout must be positive"))
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

// listenLinkURL is empty for an address a listener outside could not
// open.
func listenLinkURL(publicURL string) string {
	if publicURL == "" {
		return ""
	}
	u, err := url.Parse(publicURL)
	if err != nil || u.Host == "" {
		slog.Warn("listen_links_off", "reason", "navidrome.public_url is not a URL", "public_url", publicURL)
		return ""
	}
	host := strings.ToLower(u.Hostname())
	ip := net.ParseIP(host)
	local := ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()) ||
		ip == nil && (!strings.Contains(host, ".") || hasLocalSuffix(host))
	if local {
		slog.Warn("listen_links_off", "reason", "navidrome.public_url is not reachable from the internet", "public_url", publicURL)
		return ""
	}
	return strings.TrimSuffix(publicURL, "/")
}

var localSuffixes = []string{".localhost", ".local", ".lan", ".internal", ".home.arpa"}

func hasLocalSuffix(host string) bool {
	for _, suffix := range localSuffixes {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	return false
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

var quotaUnits = map[string]int64{"KB": 1 << 10, "MB": 1 << 20, "GB": 1 << 30, "TB": 1 << 40}

// parseQuota reads a size like "10GB" or "1.5 TB", units binary, and an
// empty value as unlimited.
func parseQuota(key, value string, problems *[]error) library.Quota {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return library.Unlimited
	}
	for unit, bytes := range quotaUnits {
		number, ok := strings.CutSuffix(value, unit)
		if !ok {
			continue
		}
		n, err := strconv.ParseFloat(strings.TrimSpace(number), 64)
		if err == nil && n*float64(bytes) >= 1 {
			return library.Quota(n * float64(bytes))
		}
	}
	*problems = append(*problems, fmt.Errorf(`%s: %q is not like "10GB" or "500MB"`, key, value))
	return library.Unlimited
}
