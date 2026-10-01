// Invite: a one-time code that lets a person join the service as a User.

package access

import (
	"errors"
	"time"
)

// Invite is an aggregate root: a code that lets one person join. It works
// once and only until ExpiresAt.
type Invite struct {
	Code      string `gorm:"primaryKey"`
	CreatedBy uint   `gorm:"not null"`
	CreatedAt time.Time
	ExpiresAt time.Time `gorm:"not null"`
	UsedBy    *uint
	UsedAt    *time.Time
}

var (
	ErrNotAdmin      = errors.New("not an admin")
	ErrInviteInvalid = errors.New("invite is unknown, used or expired")
)

func NewInvite(code string, creator *User, now time.Time, ttl time.Duration) (*Invite, error) {
	if err := creator.RequireAdmin(); err != nil {
		return nil, err
	}
	return &Invite{Code: code, CreatedBy: creator.ID, CreatedAt: now, ExpiresAt: now.Add(ttl)}, nil
}

func (i *Invite) Redeem(user *User, now time.Time) error {
	if i.UsedAt != nil || !now.Before(i.ExpiresAt) {
		return ErrInviteInvalid
	}
	i.UsedBy = &user.ID
	i.UsedAt = &now
	return nil
}
