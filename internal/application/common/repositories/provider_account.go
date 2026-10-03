package repositories

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/beatstash/internal/domain/provider"
)

type ProviderAccounts interface {
	Get(ctx context.Context, userID uint, providerName provider.ProviderName) (*provider.ProviderAccount, error)
	// GetForUpdate locks the account until ctx's transaction ends.
	GetForUpdate(ctx context.Context, userID uint, providerName provider.ProviderName) (*provider.ProviderAccount, error)
	All(ctx context.Context, providerName provider.ProviderName) ([]provider.ProviderAccount, error)
	Save(ctx context.Context, account *provider.ProviderAccount) error
	Invalid(ctx context.Context) ([]provider.ProviderAccount, error)
}

var ErrProviderAccountNotFound = errors.New("provider account not found")
