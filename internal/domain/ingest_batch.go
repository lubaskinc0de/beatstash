package domain

import "time"

type IngestBatchKind string

const (
	IngestBatchImport IngestBatchKind = "import"
)

type IngestBatch struct {
	ID uint `gorm:"primaryKey"`

	UserID uint `gorm:"not null;index"`
	User   User `gorm:"constraint:OnDelete:CASCADE;"`

	Provider ProviderName    `gorm:"not null"`
	Kind     IngestBatchKind `gorm:"not null"`
	Total    int             `gorm:"not null"`

	ChatID int64 `gorm:"not null"`
	// MessageID is the progress message.
	MessageID int `gorm:"not null"`

	CreatedAt  time.Time
	FinishedAt *time.Time `gorm:"index"`
}
