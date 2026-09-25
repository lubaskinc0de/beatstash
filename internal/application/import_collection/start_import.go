package import_collection

import (
	"context"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/accounts"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/providers"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
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
	Reporter  common.BatchReporter
	Waker     common.Waker
}

func (i *StartImport) Execute(ctx context.Context, provider domain.ProviderName, chatID int64) (*Plan, error) {
	user, err := i.IDs.CurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	running, err := i.BatchRepo.Running(ctx, user.ID, provider, domain.IngestBatchImport)
	if err != nil {
		return nil, err
	}
	if running {
		return nil, accounts.ErrBatchRunning
	}

	collection, missing, err := survey(ctx, i.Providers, i.Libraries, i.Tracks, user.ID, provider)
	if err != nil {
		return nil, err
	}
	// Rechecked under the lock: a second tap may have started an Import.
	remember := func(ctx context.Context) error {
		account, err := accounts.LockIdle(ctx, i.Accounts, i.BatchRepo, user.ID, provider)
		if err != nil {
			return err
		}
		account.Remember(collection.Snapshot(), time.Now())
		return i.Accounts.Save(ctx, account)
	}
	if len(missing) == 0 {
		err = i.Tx.WithinTx(ctx, remember)
	} else {
		err = i.startBatch(ctx, user.ID, provider, chatID, missing, remember)
	}
	if err != nil {
		return nil, err
	}
	return planOf(collection.Tracks(), missing), nil
}

// startBatch announces first: jobs may finish before a later message.
func (i *StartImport) startBatch(
	ctx context.Context,
	userID uint,
	provider domain.ProviderName,
	chatID int64,
	tracks []providers.ListedTrack,
	remember func(context.Context) error,
) error {
	batch := &domain.IngestBatch{
		UserID:   userID,
		Provider: provider,
		Kind:     domain.IngestBatchImport,
		ChatID:   chatID,
		Total:    len(tracks),
	}
	id, err := i.Reporter.Announce(ctx, batch)
	if err != nil {
		return err
	}
	batch.MessageID = id

	err = i.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := remember(ctx); err != nil {
			return err
		}
		if err := i.BatchRepo.Save(ctx, batch); err != nil {
			return err
		}
		for _, track := range tracks {
			job := common.NewJob(userID, track, common.MessageRef{ChatID: chatID})
			job.BatchID = &batch.ID
			if err := i.Queue.Enqueue(ctx, job); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		i.Reporter.Withdraw(ctx, batch)
		return err
	}
	i.Waker.Wake()
	return nil
}
