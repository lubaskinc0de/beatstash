package access

import "time"

// User is an aggregate root. Its Identities are part of it.
type User struct {
	ID uint `gorm:"primaryKey"`

	Username string
	// Admin is set and removed from the config on every start.
	Admin     bool `gorm:"not null;default:false"`
	CreatedAt time.Time

	Identities []Identity `gorm:"constraint:OnDelete:CASCADE;"`
}

// Identity is a value object: the User's account in one Channel.
type Identity struct {
	Channel    Channel `gorm:"primaryKey"`
	ExternalID string  `gorm:"primaryKey"`
	UserID     uint    `gorm:"not null;index"`
}

// Channel is a value object: where the User talks to the bot, e.g. Telegram.
type Channel string

// NewUser creates a User with its first Identity: every User must have
// one.
func NewUser(username string, identity Identity, now time.Time) *User {
	return &User{Username: username, CreatedAt: now, Identities: []Identity{identity}}
}

func (u *User) Rename(username string) {
	u.Username = username
}

func (u *User) SetAdmin(admin bool) {
	u.Admin = admin
}
