// NavidromeAccount: the login and password the bot acts with in Navidrome
// for a User. The password is encrypted with SECRET_KEY, not hashed:
// Subsonic needs it in plain text to log in.

package access

import "time"

// NavidromeAccount is an aggregate root: the login and password the bot uses
// to act in Navidrome for the User.
type NavidromeAccount struct {
	UserID   uint   `gorm:"primaryKey"`
	Login    string `gorm:"not null"`
	Password []byte `gorm:"not null"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
