package app

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Token     string
	BotApiUrl string
	DbDsn     string
	MusicDir  string
	// NavidromeMusicDir is MUSIC_DIR as Navidrome's container sees it.
	NavidromeMusicDir string
	AdminIds          []uint64
	AdminContact      string
	// SecretKey is a base64-encoded 32-byte AES key for stored secrets.
	SecretKey         string
	NavidromeUser     string
	NavidromePassword string
	NavidromeUrl      string

	InviteTTL time.Duration
	Clock     func() time.Time

	IngestWorkers int
	// IngestRetryDelays are waits before each retry of a failed Ingest Job.
	IngestRetryDelays  []time.Duration
	IngestPollInterval time.Duration
}

func LoadConfig() Config {
	token := os.Getenv("BOT_TOKEN")
	botApiUrl := os.Getenv("BOT_API_URL")
	dbDsn := os.Getenv("DB_DSN")
	musicDir := os.Getenv("MUSIC_DIR")
	navidromeMusicDir := os.Getenv("NAVIDROME_MUSIC_DIR")
	if navidromeMusicDir == "" {
		navidromeMusicDir = musicDir
	}
	secretKey := os.Getenv("SECRET_KEY")
	navidromeUser := os.Getenv("NAVIDROME_USER")
	navidomePass := os.Getenv("NAVIDROME_PASSWORD")
	navidromeUrl := os.Getenv("NAVIDROME_URL")

	adminIds := parseIds("ADMIN_IDS")
	adminContact := strings.TrimSpace(os.Getenv("ADMIN_CONTACT"))

	switch {
	case token == "":
		exitMissing("BOT_TOKEN")
	case dbDsn == "":
		exitMissing("DB_DSN")
	case musicDir == "":
		exitMissing("MUSIC_DIR")
	case len(adminIds) == 0:
		exitMissing("ADMIN_IDS")
	case secretKey == "":
		exitMissing("SECRET_KEY")
	case navidromeUrl == "":
		exitMissing("NAVIDROME_URL")
	case navidromeUser == "":
		exitMissing("NAVIDROME_USER")
	case navidomePass == "":
		exitMissing("NAVIDROME_PASSWORD")
	}

	return Config{
		Token:             token,
		BotApiUrl:         botApiUrl,
		DbDsn:             dbDsn,
		MusicDir:          musicDir,
		NavidromeMusicDir: navidromeMusicDir,
		AdminIds:          adminIds,
		AdminContact:      adminContact,
		SecretKey:         secretKey,
		NavidromeUser:     navidromeUser,
		NavidromePassword: navidomePass,
		NavidromeUrl:      navidromeUrl,

		InviteTTL: 7 * 24 * time.Hour,
		Clock:     time.Now,

		IngestWorkers:      2,
		IngestRetryDelays:  []time.Duration{10 * time.Second, time.Minute, 5 * time.Minute},
		IngestPollInterval: time.Second,
	}
}

func parseIds(field string) []uint64 {
	raw := os.Getenv(field)
	if raw == "" {
		return nil
	}

	var ids []uint64
	for _, s := range strings.Split(raw, ",") {
		id, err := strconv.ParseUint(strings.TrimSpace(s), 10, 64)
		if err != nil {
			slog.Error("config_value_invalid", "field", field, "reason", "not_an_integer")
			os.Exit(1)
		}
		ids = append(ids, id)
	}
	return ids
}

func exitMissing(field string) {
	slog.Error("missing_config_value", "field", field)
	os.Exit(1)
}
