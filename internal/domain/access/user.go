// User: a person allowed to use the service, the Identities they are known
// by, their names and their Last Seen. Admin is set or removed from the
// config on every start. LastSeenAt is nil until the User's first request
// after joining.

package access

import "time"

// User is an aggregate root. Its Identities are part of it.
type User struct {
	ID uint `gorm:"primaryKey"`

	Username   string
	FirstName  string
	LastName   string
	Admin      bool `gorm:"not null;default:false"`
	CreatedAt  time.Time
	LastSeenAt *time.Time

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

// Profile is a value object: the names the User has in a Channel.
type Profile struct {
	Username  string
	FirstName string
	LastName  string
}

const (
	// lastSeenStep: Last Seen is saved at most this often, not on every request.
	lastSeenStep = time.Minute
	// activeWindow: a User seen within it is active.
	activeWindow = 7 * 24 * time.Hour
)

// NewUser creates a User with its first Identity: every User must have
// one.
func NewUser(profile Profile, identity Identity, now time.Time) *User {
	u := &User{CreatedAt: now, Identities: []Identity{identity}}
	u.rename(profile)
	return u
}

// Seen records a request of the User: their profile as the Channel has it
// now, and Last Seen. It returns false when nothing worth saving changed.
func (u *User) Seen(profile Profile, now time.Time) (changed bool) {
	changed = u.Profile() != profile
	u.rename(profile)
	if u.LastSeenAt == nil || now.Sub(*u.LastSeenAt) >= lastSeenStep {
		u.LastSeenAt = &now
		changed = true
	}
	return changed
}

func (u *User) Profile() Profile {
	return Profile{Username: u.Username, FirstName: u.FirstName, LastName: u.LastName}
}

func (u *User) SetAdmin(admin bool) {
	u.Admin = admin
}

// ActiveSince is when a User seen then or later counts as active now.
func ActiveSince(now time.Time) time.Time {
	return now.Add(-activeWindow)
}

func (u *User) RequireAdmin() error {
	if !u.Admin {
		return ErrNotAdmin
	}
	return nil
}

func (u *User) rename(p Profile) {
	u.Username, u.FirstName, u.LastName = p.Username, p.FirstName, p.LastName
}
