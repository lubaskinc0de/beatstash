package harness

import (
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/navidrome"
)

type User struct {
	ID       int64
	Username string
}

var (
	Admin    = User{ID: 1000, Username: "boss"}
	Alice    = User{ID: 1001, Username: "alice"}
	Bob      = User{ID: 1002, Username: "bob"}
	Stranger = User{ID: 6666, Username: "mallory"}
)

// Newcomer is a Telegram user unknown to the bot, with a username free in Navidrome.
func Newcomer(name string) User {
	return User{ID: 2000 + newcomerSeq.Add(1), Username: navidrome.UniqueLogin(name)}
}
