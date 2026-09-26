package import_collection

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/providers"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
)

// ImportSource is a Provider that can give out a Provider Collection.
type ImportSource struct {
	Provider provider.ProviderName
	Status   SourceStatus
}

type SourceStatus string

const (
	SourceNotConnected  SourceStatus = "not_connected"
	SourceConnected     SourceStatus = "connected"
	SourceTokenRejected SourceStatus = "token_rejected"
)

type ListImportSources struct {
	IDs       common.IDProvider
	Providers *providers.Registry
	Accounts  repositories.ProviderAccounts
}

func (i *ListImportSources) Execute(ctx context.Context) ([]ImportSource, error) {
	user, err := i.IDs.CurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	names := i.Providers.CollectionListers()
	sources := make([]ImportSource, 0, len(names))
	for _, name := range names {
		account, err := i.Accounts.Get(ctx, user.ID, name)
		switch {
		case errors.Is(err, repositories.ErrProviderAccountNotFound):
			sources = append(sources, ImportSource{Provider: name, Status: SourceNotConnected})
			continue
		case err != nil:
			return nil, err
		}
		sources = append(sources, ImportSource{Provider: name, Status: statusOf(account)})
	}
	return sources, nil
}

func statusOf(account *provider.ProviderAccount) SourceStatus {
	_, err := account.UsableToken()
	switch {
	case errors.Is(err, provider.ErrTokenRejected):
		return SourceTokenRejected
	case err != nil:
		return SourceNotConnected
	default:
		return SourceConnected
	}
}
