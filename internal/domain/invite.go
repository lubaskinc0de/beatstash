package domain

import "time"

type Invite struct {
	Code      string `gorm:"primaryKey"`
	CreatedBy uint   `gorm:"not null"`
	CreatedAt time.Time
	ExpiresAt time.Time `gorm:"not null"`
	UsedBy    *uint
	UsedAt    *time.Time
}

func (i *Invite) Redeemable(now time.Time) bool {
	return i.UsedAt == nil && now.Before(i.ExpiresAt)
}
