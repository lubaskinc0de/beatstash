package add_track

import (
	"context"
	"errors"
	"slices"

	"github.com/lubaskinc0de/beatstash/internal/application/common"
	"github.com/lubaskinc0de/beatstash/internal/application/common/quotas"
	"github.com/lubaskinc0de/beatstash/internal/application/common/repositories"
	"github.com/lubaskinc0de/beatstash/internal/domain/ingest"
	"github.com/lubaskinc0de/beatstash/internal/domain/library"
)

var ErrIngestJobNotFound = errors.New("ingest job not found")

type IngestResults struct {
	Jobs []ingest.IngestJob
	// Usage of the Personal Library is set when a job did not fit its Quota.
	Usage library.Usage
}

type GetIngestResults struct {
	IDs    common.IDProvider
	Queue  repositories.IngestQueue
	Quotas *quotas.Quotas
}

// Execute fails with ErrIngestJobNotFound if any of the jobs is not the
// User's.
func (i *GetIngestResults) Execute(ctx context.Context, ids []uint) (*IngestResults, error) {
	user, err := i.IDs.CurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	jobs, err := i.Queue.GetOf(ctx, user.ID, ids)
	if err != nil {
		return nil, err
	}
	if len(jobs) != len(ids) {
		return nil, ErrIngestJobNotFound
	}
	results := &IngestResults{Jobs: jobs}
	if !slices.ContainsFunc(jobs, func(job ingest.IngestJob) bool { return job.FailureReason == ingest.ReasonQuotaExceeded }) {
		return results, nil
	}
	results.Usage, err = i.Quotas.PersonalUsage(ctx, user.ID)
	return results, err
}
