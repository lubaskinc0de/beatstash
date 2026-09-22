package entities

import "time"

type Track struct {
	ID uint `gorm:"primaryKey"`

	TelegramFileID       string `gorm:"not null"`
	TelegramFileUniqueID string `gorm:"uniqueIndex;not null"`

	FileName string
	MimeType string
	FileSize int64
	Duration int

	Title     string
	Performer string

	MessageId int

	SavedToPath string

	UserID uint `gorm:"not null"`
	User   User `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	CreatedAt time.Time
}
