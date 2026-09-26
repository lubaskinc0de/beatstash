package harness

import (
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/navidrome"
)

type User struct {
	ID           int64
	Username     string
	LanguageCode string
}

var (
	Admin    = User{ID: 1000, Username: "boss", LanguageCode: "ru"}
	Alice    = User{ID: 1001, Username: "alice", LanguageCode: "ru"}
	Bob      = User{ID: 1002, Username: "bob", LanguageCode: "ru"}
	Stranger = User{ID: 6666, Username: "mallory", LanguageCode: "ru"}
)

// Newcomer is a Telegram user unknown to the bot, with a username free in Navidrome.
func Newcomer(name string) User {
	return User{ID: 2000 + newcomerSeq.Add(1), Username: navidrome.UniqueLogin(name), LanguageCode: "ru"}
}

func (u User) telegram() *models.User {
	return &models.User{ID: u.ID, Username: u.Username, LanguageCode: u.LanguageCode}
}
