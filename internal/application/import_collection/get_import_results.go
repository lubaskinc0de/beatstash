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
	// FailedNames and OverQuota are set once the Import is finished. A
	// track that awaits room in the Library is counted, not named: there
	// may be thousands.
	FailedNames []string
	OverQuota   int
}

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
	failures := map[uint][]repositories.Failure{}
	if len(finished) > 0 {
		if failures, err = i.Queue.Failures(ctx, finished); err != nil {
			return nil, err
		}
	}

	results := make([]ImportResult, 0, len(batches))
	for n, batch := range batches {
		result := ImportResult{ImportProgress: progress[n], ID: batch.ID, Finished: batch.Finished()}
		for _, failure := range failures[batch.ID] {
			if failure.Reason.AwaitsRoom() {
				result.OverQuota++
			} else {
				result.FailedNames = append(result.FailedNames, failure.DisplayName)
			}
		}
		results = append(results, result)
	}
	return results, nil
}
