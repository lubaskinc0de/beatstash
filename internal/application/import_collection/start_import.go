package import_collection

import (
	"context"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/accounts"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/providers"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/ingest"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
)

type StartImport struct {
	IDs       common.IDProvider
	Tx        repositories.TxManager
	Providers *providers.Registry
	Libraries repositories.Libraries
	Tracks    repositories.Tracks
	Accounts  repositories.ProviderAccounts
	Queue     repositories.IngestQueue
	BatchRepo repositories.IngestBatches
	Waker     common.Waker
}

func (i *StartImport) Execute(ctx context.Context, providerName provider.ProviderName) (*Plan, error) {
	user, err := i.IDs.CurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	running, err := i.BatchRepo.Running(ctx, user.ID, providerName, ingest.IngestBatchImport)
	if err != nil {
		return nil, err
	}
	if running {
		return nil, accounts.ErrBatchRunning
	}

	collection, missing, err := survey(ctx, i.Providers, i.Libraries, i.Tracks, user.ID, providerName)
	if err != nil {
		return nil, err
	}
	// Rechecked under the lock: a second tap may have started an Import.
	remember := func(ctx context.Context) error {
		account, err := accounts.LockIdle(ctx, i.Accounts, i.BatchRepo, user.ID, providerName)
		if err != nil {
			return err
		}
		account.Remember(collection.Snapshot(), time.Now())
		return i.Accounts.Save(ctx, account)
	}
	plan := planOf(collection.Tracks(), missing)
	if len(missing) == 0 {
		err = i.Tx.WithinTx(ctx, remember)
	} else {
		plan.BatchID, err = i.startBatch(ctx, user.ID, providerName, missing, remember)
	}
	if err != nil {
		return nil, err
	}
	return plan, nil
}

func (i *StartImport) startBatch(
	ctx context.Context,
	userID uint,
	providerName provider.ProviderName,
	tracks []providers.ListedTrack,
	remember func(context.Context) error,
) (uint, error) {
	batch := &ingest.IngestBatch{
		UserID:   userID,
		Provider: providerName,
		Kind:     ingest.IngestBatchImport,
		Total:    len(tracks),
	}
	err := i.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := remember(ctx); err != nil {
			return err
		}
		if err := i.BatchRepo.Save(ctx, batch); err != nil {
			return err
		}
		for _, track := range tracks {
			job := batch.NewJob(track.Ref, track.DisplayName, time.Now())
			if err := i.Queue.Enqueue(ctx, job); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	i.Waker.Wake()
	return batch.ID, nil
}
