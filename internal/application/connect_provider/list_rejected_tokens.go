package connect_provider

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
)

// ListRejectedTokens lets a Channel tell users their Provider stopped
// accepting the token. The system asks it: whose token was rejected is not
// known beforehand.
type ListRejectedTokens struct {
	Accounts repositories.ProviderAccounts
}

// Execute lists the accounts invalid now, each with when it turned so.
func (i *ListRejectedTokens) Execute(ctx context.Context) ([]provider.ProviderAccount, error) {
	return i.Accounts.Invalid(ctx)
}
