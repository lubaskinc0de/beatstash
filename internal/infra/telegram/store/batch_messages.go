package store

import (
	"context"

	"gorm.io/gorm"
)

type BatchMessage struct {
	BatchID   uint  `gorm:"primaryKey;autoIncrement:false"`
	ChatID    int64 `gorm:"not null"`
	MessageID int   `gorm:"not null"`
	// Shown is the text the message has, so it is edited only on a change.
	Shown string `gorm:"not null"`
}

func (BatchMessage) TableName() string {
	return "telegram_batch_messages"
}

type BatchMessages struct {
	DB *gorm.DB
}

func (m *BatchMessages) Remember(ctx context.Context, message BatchMessage) error {
	return m.DB.WithContext(ctx).Create(&message).Error
}

func (m *BatchMessages) All(ctx context.Context) ([]BatchMessage, error) {
	var messages []BatchMessage
	err := m.DB.WithContext(ctx).Order("batch_id").Find(&messages).Error
	return messages, err
}

func (m *BatchMessages) Shown(ctx context.Context, batchID uint, text string) error {
	return m.DB.WithContext(ctx).Model(&BatchMessage{}).Where("batch_id = ?", batchID).Update("shown", text).Error
}

func (m *BatchMessages) Forget(ctx context.Context, batchID uint) error {
	return m.DB.WithContext(ctx).Delete(&BatchMessage{}, batchID).Error
}
