package database

import (
	"context"

	"gorm.io/gorm"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
)

type UserRepository struct {
	DB *gorm.DB
}

func (r *UserRepository) GetByIdentity(ctx context.Context, identity access.Identity) (*access.User, error) {
	q := dbForContext(ctx, r.DB).
		Preload("Identities").
		Where("id = (?)", dbForContext(ctx, r.DB).
			Model(&access.Identity{}).
			Select("user_id").
			Where("channel = ? AND external_id = ?", identity.Channel, identity.ExternalID),
		)
	return first[access.User](q, repositories.ErrUserNotFound)
}

func (r *UserRepository) Identities(ctx context.Context, channel access.Channel, userIDs []uint) ([]access.Identity, error) {
	var identities []access.Identity
	err := dbForContext(ctx, r.DB).Where("channel = ? AND user_id IN ?", channel, userIDs).Find(&identities).Error
	return identities, err
}

func (r *UserRepository) All(ctx context.Context) ([]access.User, error) {
	var users []access.User
	err := dbForContext(ctx, r.DB).Preload("Identities").Order("id").Find(&users).Error
	return users, err
}

func (r *UserRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := dbForContext(ctx, r.DB).Model(&access.User{}).Count(&count).Error
	return count, err
}

func (r *UserRepository) Save(ctx context.Context, user *access.User) error {
	return dbForContext(ctx, r.DB).Save(user).Error
}

func (r *UserRepository) SetUsername(ctx context.Context, userID uint, username string) error {
	return dbForContext(ctx, r.DB).
		Model(&access.User{}).
		Where("id = ?", userID).
		Update("username", username).Error
}
