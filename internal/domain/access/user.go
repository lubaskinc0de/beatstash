package access

import "time"

type User struct {
	ID uint `gorm:"primaryKey"`

	Username string
	// Admin is granted and taken back by the config on every start.
	Admin     bool `gorm:"not null;default:false"`
	CreatedAt time.Time

	Identities []Identity `gorm:"constraint:OnDelete:CASCADE;"`
}

type Channel string

type Identity struct {
	Channel    Channel `gorm:"primaryKey"`
	ExternalID string  `gorm:"primaryKey"`
	UserID     uint    `gorm:"not null;index"`
}

// NewUser: a User is always born known to some Channel.
func NewUser(username string, identity Identity, now time.Time) *User {
	return &User{Username: username, CreatedAt: now, Identities: []Identity{identity}}
}

func (u *User) SetAdmin(admin bool) {
	u.Admin = admin
}
