package import_collection

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
)

type RunningImport struct {
	Provider provider.ProviderName
	Total    int
	Progress repositories.BatchProgress
}

type GetRunningImports struct {
	IDs     common.IDProvider
	Queue   repositories.IngestQueue
	Batches repositories.IngestBatches
}

// Execute lists them oldest first.
func (i *GetRunningImports) Execute(ctx context.Context) ([]RunningImport, error) {
	user, err := i.IDs.CurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	batches, err := i.Batches.UnfinishedOf(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(batches))
	for _, batch := range batches {
		ids = append(ids, batch.ID)
	}
	progress, err := i.Queue.BatchProgress(ctx, ids)
	if err != nil {
		return nil, err
	}
	running := make([]RunningImport, 0, len(batches))
	for _, batch := range batches {
		running = append(running, RunningImport{Provider: batch.Provider, Total: batch.Total, Progress: progress[batch.ID]})
	}
	return running, nil
}
