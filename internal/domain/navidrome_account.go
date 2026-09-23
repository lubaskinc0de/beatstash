package domain

import "time"

type NavidromeAccount struct {
	UserID uint   `gorm:"primaryKey"`
	Login  string `gorm:"not null"`
	// Password is sealed with SECRET_KEY: Subsonic auth needs it in plain text.
	Password []byte `gorm:"not null"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
