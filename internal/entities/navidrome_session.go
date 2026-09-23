package entities

import "time"

type NavidromeSession struct {
	Username string `gorm:"primaryKey"`
	Token    string `gorm:"not null"`

	UpdatedAt time.Time
}
