package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Token             string
	DbDsn             string
	MusicDir          string
	AllowedUserIds    []uint64
	NavidromeUser     string
	NavidromePassword string
	NavidromeUrl      string
}

func LoadConfig() Config {
	token := os.Getenv("BOT_TOKEN")
	dbDsn := os.Getenv("DB_DSN")
	musicDir := os.Getenv("MUSIC_DIR")
	navidromeUser := os.Getenv("NAVIDROME_USER")
	navidomePass := os.Getenv("NAVIDROME_PASSWORD")
	navidromeUrl := os.Getenv("NAVIDROME_URL")

	allowedUserIdsStrings := strings.Split(os.Getenv("ALLOWED_USER_IDS"), ",")

	allowedUserIds := []uint64{}
	if allowedUserIdsStrings[0] == "" {
		slog.Error("missing_config_value", "field", "ALLOWED_USER_IDS")
		os.Exit(1)
	}

	for _, user_id_str := range allowedUserIdsStrings {
		user_id, err := strconv.Atoi(user_id_str)
		if err != nil {
			slog.Error("config_value_invalid", "field", "ALLOWED_USER_IDS", "reason", "not_an_integer")
			os.Exit(1)
		}
		allowedUserIds = append(allowedUserIds, uint64(user_id))
	}

	switch {
	case token == "":
		slog.Error("missing_config_value", "field", "BOT_TOKEN")
		os.Exit(1)
	case dbDsn == "":
		slog.Error("missing_config_value", "field", "DB_DSN")
		os.Exit(1)
	case musicDir == "":
		slog.Error("missing_config_value", "field", "MUSIC_DIR")
		os.Exit(1)
	case navidromeUrl == "":
		slog.Error("missing_config_value", "field", "NAVIDROME_URL")
		os.Exit(1)
	case navidromeUser == "":
		slog.Error("missing_config_value", "field", "NAVIDROME_USER")
		os.Exit(1)
	case navidomePass == "":
		slog.Error("missing_config_value", "field", "NAVIDROME_PASS")
		os.Exit(1)
	}

	return Config{
		Token:             token,
		DbDsn:             dbDsn,
		MusicDir:          musicDir,
		AllowedUserIds:    allowedUserIds,
		NavidromeUser:     navidromeUser,
		NavidromePassword: navidomePass,
		NavidromeUrl:      navidromeUrl,
	}
}
