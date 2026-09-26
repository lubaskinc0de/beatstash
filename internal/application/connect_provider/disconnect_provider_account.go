package connect_provider

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
)

type DisconnectProviderAccount struct {
	IDs      common.IDProvider
	Tx       repositories.TxManager
	Accounts repositories.ProviderAccounts
}

func (i *DisconnectProviderAccount) Execute(ctx context.Context, providerName provider.ProviderName) error {
	user, err := i.IDs.CurrentUser(ctx)
	if err != nil {
		return err
	}
	return i.Tx.WithinTx(ctx, func(ctx context.Context) error {
		account, err := i.Accounts.GetForUpdate(ctx, user.ID, providerName)
		if errors.Is(err, repositories.ErrProviderAccountNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		account.Disconnect()
		return i.Accounts.Save(ctx, account)
	})
}
