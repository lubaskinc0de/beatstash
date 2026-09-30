package repositories

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/ingest"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
)

type IngestQueue interface {
	Enqueue(ctx context.Context, jobs ...*ingest.IngestJob) error
	ClaimNext(ctx context.Context, filter JobFilter) (*ingest.IngestJob, error)
	Save(ctx context.Context, job *ingest.IngestJob) error
	// GetOf leaves out the jobs that are not the user's.
	GetOf(ctx context.Context, userID uint, ids []uint) ([]ingest.IngestJob, error)
	CountUnfinished(ctx context.Context) (int64, error)
	// BatchProgress maps each batch to its progress.
	BatchProgress(ctx context.Context, batchIDs []uint) (map[uint]BatchProgress, error)
	// FailedNames maps each batch to the names of its failed jobs, oldest first.
	FailedNames(ctx context.Context, batchIDs []uint) (map[uint][]string, error)
	PendingRefs(ctx context.Context, userID uint, providerName provider.ProviderName) ([]string, error)
}

type JobFilter struct {
	Only   []provider.ProviderName
	Except []provider.ProviderName
	// PerUser caps how many jobs of one user run at once; zero lifts the cap.
	PerUser int
}

type BatchProgress struct {
	Done    int
	Failed  int
	Pending int
}
