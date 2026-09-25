package repositories

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type ProviderAccounts interface {
	Get(ctx context.Context, userID uint, provider domain.ProviderName) (*domain.ProviderAccount, error)
	// GetForUpdate locks the account until ctx's transaction ends.
	GetForUpdate(ctx context.Context, userID uint, provider domain.ProviderName) (*domain.ProviderAccount, error)
	All(ctx context.Context, provider domain.ProviderName) ([]domain.ProviderAccount, error)
	Save(ctx context.Context, account *domain.ProviderAccount) error
}

var ErrProviderAccountNotFound = errors.New("provider account not found")
