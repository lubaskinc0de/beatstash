package database

import (
	"context"
	"time"

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

func (r *UserRepository) CountActive(ctx context.Context, since time.Time) (total, active int64, err error) {
	var row struct {
		Total  int64
		Active int64
	}
	err = dbForContext(ctx, r.DB).
		Model(&access.User{}).
		Select("COUNT(*) AS total, COUNT(*) FILTER (WHERE last_seen_at >= ?) AS active", since).
		Scan(&row).Error
	return row.Total, row.Active, err
}

// ByLastSeen reads one row past the page: it tells whether more follow.
func (r *UserRepository) ByLastSeen(ctx context.Context, page, perPage int) ([]access.User, bool, error) {
	var users []access.User
	err := dbForContext(ctx, r.DB).
		Order("last_seen_at DESC NULLS LAST, id").
		Offset(page * perPage).Limit(perPage + 1).
		Find(&users).Error
	if err != nil || len(users) <= perPage {
		return users, false, err
	}
	return users[:perPage], true, nil
}

func (r *UserRepository) Get(ctx context.Context, id uint) (*access.User, error) {
	return first[access.User](dbForContext(ctx, r.DB).Preload("Identities").Where("id = ?", id), repositories.ErrUserNotFound)
}

func (r *UserRepository) Save(ctx context.Context, user *access.User) error {
	return dbForContext(ctx, r.DB).Save(user).Error
}

func (r *UserRepository) SaveSeen(ctx context.Context, user *access.User) error {
	return dbForContext(ctx, r.DB).
		Model(&access.User{}).
		Where("id = ?", user.ID).
		Updates(map[string]any{
			"username":     user.Username,
			"first_name":   user.FirstName,
			"last_name":    user.LastName,
			"last_seen_at": user.LastSeenAt,
		}).Error
}
