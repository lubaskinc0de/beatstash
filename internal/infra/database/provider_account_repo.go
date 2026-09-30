package database

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
)

type ProviderAccountRepository struct {
	DB *gorm.DB
}

func (r *ProviderAccountRepository) Get(
	ctx context.Context,
	userID uint,
	providerName provider.ProviderName,
) (*provider.ProviderAccount, error) {
	q := dbForContext(ctx, r.DB).Where("user_id = ? AND provider = ?", userID, providerName)
	return first[provider.ProviderAccount](q, repositories.ErrProviderAccountNotFound)
}

func (r *ProviderAccountRepository) GetForUpdate(
	ctx context.Context,
	userID uint,
	providerName provider.ProviderName,
) (*provider.ProviderAccount, error) {
	q := dbForContext(ctx, r.DB).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND provider = ?", userID, providerName)
	return first[provider.ProviderAccount](q, repositories.ErrProviderAccountNotFound)
}

func (r *ProviderAccountRepository) All(ctx context.Context, providerName provider.ProviderName) ([]provider.ProviderAccount, error) {
	var accounts []provider.ProviderAccount
	err := dbForContext(ctx, r.DB).Preload("User").Where("provider = ?", providerName).Order("user_id").Find(&accounts).Error
	return accounts, err
}

func (r *ProviderAccountRepository) Save(ctx context.Context, account *provider.ProviderAccount) error {
	return dbForContext(ctx, r.DB).
		Omit("User").
		Clauses(clause.OnConflict{UpdateAll: true}).
		Create(account).Error
}

func (r *ProviderAccountRepository) Invalid(ctx context.Context) ([]provider.ProviderAccount, error) {
	var accounts []provider.ProviderAccount
	err := dbForContext(ctx, r.DB).
		Select("user_id, provider, invalidated_at").
		Where("status = ? AND invalidated_at IS NOT NULL", provider.ProviderAccountInvalid).
		Order("user_id, provider").
		Find(&accounts).Error
	return accounts, err
}
