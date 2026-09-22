package config

import "os"

type Config struct {
	Token string
	DbDsn string
	MusicDir string
}

func LoadConfig() Config {
	return Config{
		Token: os.Getenv("BOT_TOKEN"),
		DbDsn: os.Getenv("DB_DSN"),
		MusicDir: os.Getenv("MUSIC_DIR"),
	}
}
