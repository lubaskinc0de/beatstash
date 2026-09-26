package database

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type NavidromeAccountRepository struct {
	DB *gorm.DB
}

func (r *NavidromeAccountRepository) Get(ctx context.Context, userID uint) (*access.NavidromeAccount, error) {
	q := dbForContext(ctx, r.DB).Where("user_id = ?", userID)
	return first[access.NavidromeAccount](q, repositories.ErrNavidromeAccountNotFound)
}

func (r *NavidromeAccountRepository) ByLogin(ctx context.Context, login string) (*access.NavidromeAccount, error) {
	q := dbForContext(ctx, r.DB).Where("LOWER(login) = LOWER(?)", login)
	return first[access.NavidromeAccount](q, repositories.ErrNavidromeAccountNotFound)
}

func (r *NavidromeAccountRepository) All(ctx context.Context) ([]access.NavidromeAccount, error) {
	var accounts []access.NavidromeAccount
	err := dbForContext(ctx, r.DB).Order("user_id").Find(&accounts).Error
	return accounts, err
}

func (r *NavidromeAccountRepository) Save(ctx context.Context, account *access.NavidromeAccount) error {
	return dbForContext(ctx, r.DB).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"login", "password", "updated_at"}),
		}).
		Create(account).Error
}
