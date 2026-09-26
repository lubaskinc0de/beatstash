package connect_provider

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/providers"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
)

type ConnectProviderAccount struct {
	IDs       common.IDProvider
	Tx        repositories.TxManager
	Providers *providers.Registry
	Accounts  repositories.ProviderAccounts
	Box       common.SecretBox
}

func (i *ConnectProviderAccount) Execute(ctx context.Context, providerName provider.ProviderName, token string) error {
	user, err := i.IDs.CurrentUser(ctx)
	if err != nil {
		return err
	}
	checker, err := i.Providers.TokenChecker(providerName)
	if err != nil {
		return err
	}
	if err := checker.CheckToken(ctx, token); err != nil {
		return err
	}
	sealed, err := i.Box.Seal(token)
	if err != nil {
		return err
	}

	// A reconnected account keeps its collection, so Sync resumes.
	return i.Tx.WithinTx(ctx, func(ctx context.Context) error {
		account, err := i.Accounts.GetForUpdate(ctx, user.ID, providerName)
		if errors.Is(err, repositories.ErrProviderAccountNotFound) {
			account, err = &provider.ProviderAccount{UserID: user.ID, Provider: providerName}, nil
		}
		if err != nil {
			return err
		}
		account.Connect(sealed)
		return i.Accounts.Save(ctx, account)
	})
}
