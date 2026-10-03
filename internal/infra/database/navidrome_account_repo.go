package database

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/lubaskinc0de/beatstash/internal/application/common/repositories"
	"github.com/lubaskinc0de/beatstash/internal/domain/access"
)

type NavidromeAccountRepository struct {
	DB *gorm.DB
}

func (r *NavidromeAccountRepository) Get(ctx context.Context, userID uint) (*access.NavidromeAccount, error) {
	q := dbForContext(ctx, r.DB).Where("user_id = ?", userID)
	return first[access.NavidromeAccount](q, repositories.ErrNavidromeAccountNotFound)
}

func (r *NavidromeAccountRepository) All(ctx context.Context) ([]access.NavidromeAccount, error) {
	var accounts []access.NavidromeAccount
	err := dbForContext(ctx, r.DB).Order("user_id").Find(&accounts).Error
	return accounts, err
}

func (r *NavidromeAccountRepository) Add(ctx context.Context, account *access.NavidromeAccount) error {
	return conflictOf(dbForContext(ctx, r.DB).Create(account).Error)
}

func (r *NavidromeAccountRepository) Save(ctx context.Context, account *access.NavidromeAccount) error {
	return conflictOf(dbForContext(ctx, r.DB).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"login", "password", "updated_at"}),
		}).
		Create(account).Error)
}

const uniqueViolation = "23505"

func conflictOf(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != uniqueViolation {
		return err
	}
	switch pgErr.ConstraintName {
	case "navidrome_accounts_pkey":
		return repositories.ErrNavidromeAccountExists
	case "idx_navidrome_account_login":
		return repositories.ErrNavidromeLoginLinked
	}
	return err
}
