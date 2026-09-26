package access

import "time"

// NavidromeAccount is an aggregate root: the login and password the bot uses
// to act in Navidrome for the User.
type NavidromeAccount struct {
	UserID uint   `gorm:"primaryKey"`
	Login  string `gorm:"not null"`
	// Password is encrypted with SECRET_KEY, not hashed: Subsonic needs
	// the plain password to log in.
	Password []byte `gorm:"not null"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
