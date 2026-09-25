package database

import (
	"context"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UserRepository struct {
	DB *gorm.DB
}

func (r *UserRepository) GetByTelegramID(ctx context.Context, telegramID uint64) (*domain.User, error) {
	q := dbForContext(ctx, r.DB).Where("telegram_id = ?", telegramID)
	return first[domain.User](q, application.ErrUserNotFound)
}

func (r *UserRepository) All(ctx context.Context) ([]domain.User, error) {
	var users []domain.User
	err := dbForContext(ctx, r.DB).Order("id").Find(&users).Error
	return users, err
}

func (r *UserRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := dbForContext(ctx, r.DB).Model(&domain.User{}).Count(&count).Error
	return count, err
}

func (r *UserRepository) Save(ctx context.Context, user *domain.User) error {
	return dbForContext(ctx, r.DB).Create(user).Error
}

func (r *UserRepository) EnsureExist(ctx context.Context, telegramIDs []uint64) error {
	if len(telegramIDs) == 0 {
		return nil
	}

	users := make([]domain.User, 0, len(telegramIDs))
	for _, id := range telegramIDs {
		users = append(users, domain.User{TelegramID: id, CreatedAt: time.Now()})
	}
	return dbForContext(ctx, r.DB).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "telegram_id"}}, DoNothing: true}).
		Create(&users).Error
}

func (r *UserRepository) SetUsername(ctx context.Context, userID uint, username string) error {
	return dbForContext(ctx, r.DB).
		Model(&domain.User{}).
		Where("id = ?", userID).
		Update("username", username).Error
}

func (r *UserRepository) SetAwaitsNavidromeLogin(ctx context.Context, userID uint, awaits bool) error {
	return dbForContext(ctx, r.DB).
		Model(&domain.User{}).
		Where("id = ?", userID).
		Update("awaits_navidrome_login", awaits).Error
}
