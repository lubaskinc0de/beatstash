package repositories

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/ingest"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
)

type IngestQueue interface {
	Enqueue(ctx context.Context, job *ingest.IngestJob) error
	ClaimNext(ctx context.Context, filter JobFilter) (*ingest.IngestJob, error)
	Save(ctx context.Context, job *ingest.IngestJob) error
	Get(ctx context.Context, ids []uint) ([]ingest.IngestJob, error)
	CountUnfinished(ctx context.Context) (int64, error)
	BatchProgress(ctx context.Context, batchID uint) (BatchProgress, error)
	FailedNames(ctx context.Context, batchID uint) ([]string, error)
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
