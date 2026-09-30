package import_collection

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
)

var ErrImportNotFound = errors.New("import not found")

type ImportResult struct {
	ImportProgress
	ID       uint
	Finished bool
	// FailedNames are set once the Import is finished.
	FailedNames []string
}

// GetImportResults tells the User how their Imports went.
type GetImportResults struct {
	IDs     common.IDProvider
	Queue   repositories.IngestQueue
	Batches repositories.IngestBatches
}

// Execute fails with ErrImportNotFound if any of the Imports is not the
// User's.
func (i *GetImportResults) Execute(ctx context.Context, ids []uint) ([]ImportResult, error) {
	user, err := i.IDs.CurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	batches, err := i.Batches.GetOf(ctx, user.ID, ids)
	if err != nil {
		return nil, err
	}
	if len(batches) != len(ids) {
		return nil, ErrImportNotFound
	}
	progress, err := progressOf(ctx, i.Queue, batches)
	if err != nil {
		return nil, err
	}
	var finished []uint
	for _, batch := range batches {
		if batch.Finished() {
			finished = append(finished, batch.ID)
		}
	}
	failed := map[uint][]string{}
	if len(finished) > 0 {
		if failed, err = i.Queue.FailedNames(ctx, finished); err != nil {
			return nil, err
		}
	}

	results := make([]ImportResult, 0, len(batches))
	for n, batch := range batches {
		results = append(results, ImportResult{
			ImportProgress: progress[n], ID: batch.ID, Finished: batch.Finished(), FailedNames: failed[batch.ID],
		})
	}
	return results, nil
}
