package sync_collection

import (
	"context"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
)

// GetInvalidatedProviderAccounts lets a Channel tell users their Provider
// stopped accepting the token; the system asks it, not a User.
type GetInvalidatedProviderAccounts struct {
	Accounts repositories.ProviderAccounts
}

// Execute lists the accounts still invalid that turned so at since or later.
func (i *GetInvalidatedProviderAccounts) Execute(ctx context.Context, since time.Time) ([]provider.ProviderAccount, error) {
	return i.Accounts.InvalidatedSince(ctx, since)
}
