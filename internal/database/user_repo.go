package database

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/application"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{
		db: db,
	}
}

func (r *UserRepository) GetById(ctx context.Context, telegram_id uint64) (*domain.User, error) {
	var user domain.User

	err := dbForContext(ctx, r.db).Where("telegram_id = ?", telegram_id).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, application.ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) Save(ctx context.Context, user *domain.User) error {
	return dbForContext(ctx, r.db).Create(user).Error
}
