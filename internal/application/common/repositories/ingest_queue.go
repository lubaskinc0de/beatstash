package repositories

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type IngestQueue interface {
	Enqueue(ctx context.Context, job *domain.IngestJob) error
	ClaimNext(ctx context.Context, filter JobFilter) (*domain.IngestJob, error)
	Save(ctx context.Context, job *domain.IngestJob) error
	CountUnfinished(ctx context.Context) (int64, error)
	BatchProgress(ctx context.Context, batchID uint) (BatchProgress, error)
	FailedNames(ctx context.Context, batchID uint) ([]string, error)
	PendingRefs(ctx context.Context, userID uint, provider domain.ProviderName) ([]string, error)
}

type JobFilter struct {
	Only   []domain.ProviderName
	Except []domain.ProviderName
	// PerUser caps how many jobs of one user run at once; zero lifts the cap.
	PerUser int
}

type BatchProgress struct {
	Done    int
	Failed  int
	Pending int
}
