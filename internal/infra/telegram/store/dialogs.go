package store

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Dialog struct {
	ChatID               int64 `gorm:"primaryKey;autoIncrement:false"`
	AwaitsNavidromeLogin bool  `gorm:"not null"`
}

func (Dialog) TableName() string {
	return "telegram_dialogs"
}

type Dialogs struct {
	DB *gorm.DB
}

func (d *Dialogs) AwaitNavidromeLogin(ctx context.Context, chatID int64) error {
	return d.set(ctx, chatID, true)
}

func (d *Dialogs) EndNavidromeLogin(ctx context.Context, chatID int64) error {
	return d.set(ctx, chatID, false)
}

func (d *Dialogs) AwaitsNavidromeLogin(ctx context.Context, chatID int64) (bool, error) {
	var dialogs []Dialog
	err := d.DB.WithContext(ctx).Where("chat_id = ?", chatID).Limit(1).Find(&dialogs).Error
	return len(dialogs) > 0 && dialogs[0].AwaitsNavidromeLogin, err
}

func (d *Dialogs) set(ctx context.Context, chatID int64, awaits bool) error {
	return d.DB.WithContext(ctx).
		Clauses(clause.OnConflict{UpdateAll: true}).
		Create(&Dialog{ChatID: chatID, AwaitsNavidromeLogin: awaits}).Error
}
