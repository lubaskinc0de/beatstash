package store

import (
	"context"

	"gorm.io/gorm"
)

// FollowedBatch is an Import whose summary the chat has not got yet.
type FollowedBatch struct {
	BatchID uint  `gorm:"primaryKey;autoIncrement:false"`
	ChatID  int64 `gorm:"not null"`
}

func (FollowedBatch) TableName() string {
	return "telegram_followed_batches"
}

type FollowedBatches struct {
	DB *gorm.DB
}

func (f *FollowedBatches) Follow(ctx context.Context, batch FollowedBatch) error {
	return f.DB.WithContext(ctx).Create(&batch).Error
}

func (f *FollowedBatches) All(ctx context.Context) ([]FollowedBatch, error) {
	var batches []FollowedBatch
	err := f.DB.WithContext(ctx).Order("batch_id").Find(&batches).Error
	return batches, err
}

func (f *FollowedBatches) Forget(ctx context.Context, batchID uint) error {
	return f.DB.WithContext(ctx).Delete(&FollowedBatch{}, batchID).Error
}
