package bot

import (
	"context"

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

// Languages leaves out the users who never wrote to the bot.
func (u *Users) Languages(ctx context.Context, ids []int64) (map[int64]string, error) {
	var users []User
	if err := u.DB.WithContext(ctx).Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, err
	}
	languages := make(map[int64]string, len(users))
	for _, user := range users {
		languages[user.ID] = user.Language
	}
	return languages, nil
}

// Meet keeps an already saved language.
func (u *Users) Meet(ctx context.Context, id int64, language string) error {
	return u.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&User{ID: id, Language: language}).Error
}

func (u *Users) SetLanguage(ctx context.Context, id int64, language string) error {
	return u.DB.WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).Create(&User{ID: id, Language: language}).Error
}
