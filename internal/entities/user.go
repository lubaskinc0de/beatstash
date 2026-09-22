package entities

import "time"

type User struct {
	ID uint `gorm:"primaryKey"`

	TelegramID uint64 `gorm:"uniqueIndex;not null"`
	Username   string
	CreatedAt  time.Time
}
