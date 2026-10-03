package accounts

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/beatstash/internal/application/common"
	"github.com/lubaskinc0de/beatstash/internal/application/common/providers"
	"github.com/lubaskinc0de/beatstash/internal/application/common/repositories"
	"github.com/lubaskinc0de/beatstash/internal/domain/ingest"
	"github.com/lubaskinc0de/beatstash/internal/domain/provider"
)

type ProviderTokens struct {
	Repo repositories.ProviderAccounts
	Box  common.SecretBox
}

// Token returns providers.ErrUnauthorized for an account the Provider stopped
// accepting and repositories.ErrProviderAccountNotFound for a disconnected one.
func (a *ProviderTokens) Token(ctx context.Context, userID uint, providerName provider.ProviderName) (string, error) {
	account, err := a.Repo.Get(ctx, userID, providerName)
	if err != nil {
		return "", err
	}
	sealed, err := account.UsableToken()
	switch {
	case errors.Is(err, provider.ErrAccountDisconnected):
		return "", repositories.ErrProviderAccountNotFound
	case errors.Is(err, provider.ErrTokenRejected):
		return "", providers.ErrUnauthorized
	}
	return a.Box.Open(sealed)
}

// LockIdle locks the Provider Account unless its Import is running;
// then it returns ErrBatchRunning, as the Import owns the collection.
func LockIdle(
	ctx context.Context,
	accounts repositories.ProviderAccounts,
	batches repositories.IngestBatches,
	userID uint,
	providerName provider.ProviderName,
) (*provider.ProviderAccount, error) {
	account, err := accounts.GetForUpdate(ctx, userID, providerName)
	if err != nil {
		return nil, err
	}
	running, err := batches.Running(ctx, userID, providerName, ingest.IngestBatchImport)
	if err != nil {
		return nil, err
	}
	if running {
		return nil, ErrBatchRunning
	}
	return account, nil
}

var ErrBatchRunning = errors.New("an ingest batch of the provider is running")
