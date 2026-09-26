package store

import (
	"context"
	"database/sql"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AccountNotice struct {
	UserID        uint      `gorm:"primaryKey;autoIncrement:false"`
	Provider      string    `gorm:"primaryKey"`
	InvalidatedAt time.Time `gorm:"primaryKey"`
}

func (AccountNotice) TableName() string {
	return "telegram_account_notices"
}

type AccountNotices struct {
	DB *gorm.DB
}

// Latest is the time of the last invalidation the users heard of; zero if none.
func (n *AccountNotices) Latest(ctx context.Context) (time.Time, error) {
	var latest sql.NullTime
	err := n.DB.WithContext(ctx).Model(&AccountNotice{}).Select("MAX(invalidated_at)").Row().Scan(&latest)
	return latest.Time, err
}

func (n *AccountNotices) Heard(ctx context.Context, notice AccountNotice) (bool, error) {
	var count int64
	err := n.DB.WithContext(ctx).Model(&AccountNotice{}).Where(&notice).Count(&count).Error
	return count > 0, err
}

func (n *AccountNotices) Remember(ctx context.Context, notice AccountNotice) error {
	return n.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&notice).Error
}

// ChatOf is the private chat with the user; false if they have no Telegram Identity.
func (n *AccountNotices) ChatOf(ctx context.Context, userID uint) (int64, bool, error) {
	var ids []int64
	err := n.DB.WithContext(ctx).
		Table("identities").
		Where("user_id = ? AND channel = ?", userID, "telegram").
		Pluck("external_id::bigint", &ids).Error
	if err != nil || len(ids) == 0 {
		return 0, false, err
	}
	return ids[0], true, nil
}
