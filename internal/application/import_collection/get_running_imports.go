package import_collection

import (
	"context"

	"github.com/lubaskinc0de/beatstash/internal/application/common"
	"github.com/lubaskinc0de/beatstash/internal/application/common/repositories"
)

type GetRunningImports struct {
	IDs     common.IDProvider
	Queue   repositories.IngestQueue
	Batches repositories.IngestBatches
}

// Execute lists them oldest first.
func (i *GetRunningImports) Execute(ctx context.Context) ([]ImportProgress, error) {
	user, err := i.IDs.CurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	batches, err := i.Batches.UnfinishedOf(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	return progressOf(ctx, i.Queue, batches)
}
