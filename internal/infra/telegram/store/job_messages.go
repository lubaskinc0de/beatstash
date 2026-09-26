package store

import (
	"context"

	"gorm.io/gorm"
)

// JobMessage is the message whose audio became the Ingest Job: it gets the
// job's reaction, and its file becomes the Track's.
type JobMessage struct {
	JobID     uint  `gorm:"primaryKey;autoIncrement:false"`
	ChatID    int64 `gorm:"not null"`
	MessageID int   `gorm:"not null"`

	FileID       string   `gorm:"not null"`
	FileUniqueID string   `gorm:"not null"`
	FileKind     FileKind `gorm:"not null"`
}

func (JobMessage) TableName() string {
	return "telegram_job_messages"
}

type JobMessages struct {
	DB *gorm.DB
}

func (m *JobMessages) Remember(ctx context.Context, message JobMessage) error {
	return m.DB.WithContext(ctx).Create(&message).Error
}

func (m *JobMessages) All(ctx context.Context) ([]JobMessage, error) {
	var messages []JobMessage
	err := m.DB.WithContext(ctx).Order("job_id").Find(&messages).Error
	return messages, err
}

func (m *JobMessages) Forget(ctx context.Context, jobID uint) error {
	return m.DB.WithContext(ctx).Delete(&JobMessage{}, jobID).Error
}
