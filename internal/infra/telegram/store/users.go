package store

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// User is any Telegram user who wrote to the bot, strangers included.
type User struct {
	ID       int64  `gorm:"primaryKey;autoIncrement:false"`
	Language string `gorm:"not null"`
}

func (User) TableName() string {
	return "telegram_users"
}

type Users struct {
	DB *gorm.DB
}

func (u *Users) Language(ctx context.Context, id int64) (string, bool, error) {
	var user User
	err := u.DB.WithContext(ctx).Where("id = ?", id).Take(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, nil
	}
	return user.Language, err == nil, err
}

// Meet keeps an already saved language.
func (u *Users) Meet(ctx context.Context, id int64, language string) error {
	return u.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&User{ID: id, Language: language}).Error
}

func (u *Users) SetLanguage(ctx context.Context, id int64, language string) error {
	return u.DB.WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).Create(&User{ID: id, Language: language}).Error
}
