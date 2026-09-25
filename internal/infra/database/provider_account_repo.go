package database

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type ProviderAccountRepository struct {
	DB *gorm.DB
}

func (r *ProviderAccountRepository) Get(
	ctx context.Context,
	userID uint,
	provider domain.ProviderName,
) (*domain.ProviderAccount, error) {
	q := dbForContext(ctx, r.DB).Where("user_id = ? AND provider = ?", userID, provider)
	return first[domain.ProviderAccount](q, repositories.ErrProviderAccountNotFound)
}

func (r *ProviderAccountRepository) GetForUpdate(
	ctx context.Context,
	userID uint,
	provider domain.ProviderName,
) (*domain.ProviderAccount, error) {
	q := dbForContext(ctx, r.DB).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND provider = ?", userID, provider)
	return first[domain.ProviderAccount](q, repositories.ErrProviderAccountNotFound)
}

func (r *ProviderAccountRepository) All(ctx context.Context, provider domain.ProviderName) ([]domain.ProviderAccount, error) {
	var accounts []domain.ProviderAccount
	err := dbForContext(ctx, r.DB).Preload("User").Where("provider = ?", provider).Order("user_id").Find(&accounts).Error
	return accounts, err
}

func (r *ProviderAccountRepository) Save(ctx context.Context, account *domain.ProviderAccount) error {
	return dbForContext(ctx, r.DB).
		Omit("User").
		Clauses(clause.OnConflict{UpdateAll: true}).
		Create(account).Error
}
