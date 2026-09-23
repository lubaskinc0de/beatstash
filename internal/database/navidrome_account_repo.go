package database

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/application"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type NavidromeAccountRepository struct {
	db *gorm.DB
}

func NewNavidromeAccountRepository(db *gorm.DB) *NavidromeAccountRepository {
	return &NavidromeAccountRepository{db: db}
}

func (r *NavidromeAccountRepository) Get(ctx context.Context, userID uint) (*domain.NavidromeAccount, error) {
	var account domain.NavidromeAccount

	err := dbForContext(ctx, r.db).Where("user_id = ?", userID).First(&account).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, application.ErrNavidromeAccountNotFound
	}
	if err != nil {
		return nil, err
	}
	return &account, nil
}

func (r *NavidromeAccountRepository) Save(ctx context.Context, account *domain.NavidromeAccount) error {
	return dbForContext(ctx, r.db).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"login", "password", "updated_at"}),
		}).
		Create(account).Error
}
