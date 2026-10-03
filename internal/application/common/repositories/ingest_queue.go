package repositories

import (
	"context"

	"github.com/lubaskinc0de/beatstash/internal/domain/ingest"
	"github.com/lubaskinc0de/beatstash/internal/domain/provider"
)

type IngestQueue interface {
	Enqueue(ctx context.Context, jobs ...*ingest.IngestJob) error
	ClaimNext(ctx context.Context, filter JobFilter) (*ingest.IngestJob, error)
	Save(ctx context.Context, job *ingest.IngestJob) error
	// GetOf leaves out the jobs that are not the user's.
	GetOf(ctx context.Context, userID uint, ids []uint) ([]ingest.IngestJob, error)
	CountUnfinished(ctx context.Context) (int64, error)
	BatchProgress(ctx context.Context, batchIDs []uint) (map[uint]BatchProgress, error)
	// Failures maps each batch to its failed jobs, oldest first.
	Failures(ctx context.Context, batchIDs []uint) (map[uint][]Failure, error)
	PendingRefs(ctx context.Context, userID uint, providerName provider.ProviderName) ([]string, error)
	// LatestFailures maps the refs whose latest job of the user failed to
	// the reason.
	LatestFailures(ctx context.Context, userID uint, providerName provider.ProviderName) (map[string]ingest.FailureReason, error)
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

type Failure struct {
	DisplayName string
	Reason      ingest.FailureReason
}
