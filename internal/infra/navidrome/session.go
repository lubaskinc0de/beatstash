package navidrome

import "time"

// Session caches a Navidrome token per user, so the bot does not log in on
// every request.
type Session struct {
	Username string `gorm:"primaryKey"`
	Token    string `gorm:"not null"`

	UpdatedAt time.Time
}

func (Session) TableName() string {
	return "navidrome_sessions"
}
