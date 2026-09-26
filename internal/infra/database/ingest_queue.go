package database

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/ingest"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
)

type IngestQueue struct {
	DB *gorm.DB
}

func (q *IngestQueue) Enqueue(ctx context.Context, job *ingest.IngestJob) error {
	return dbForContext(ctx, q.DB).Omit("User", "Batch").Create(job).Error
}

// ClaimNext keeps the job pending while locked, so a crashed worker's job
// becomes claimable again as soon as its transaction dies.
func (q *IngestQueue) ClaimNext(ctx context.Context, filter repositories.JobFilter) (*ingest.IngestJob, error) {
	if filter.PerUser > 0 {
		return q.claimForIdleUser(ctx, filter)
	}
	return q.claim(q.due(ctx, filter).Order("run_at, id"))
}

// userLockKey+slot is the first key of the advisory lock on a user's slot;
// libraryLockKey sits right below the range.
const userLockKey = 7_246_102

// claimForIdleUser goes through the users with due jobs, the one served
// longest ago first, and takes a job of the first user with a free slot.
// The slot is taken first, so its holder gets the user's jobs in order; a
// user whose due jobs are all taken gets the slot back by rolling back to
// the savepoint, which frees the advisory lock.
func (q *IngestQueue) claimForIdleUser(ctx context.Context, filter repositories.JobFilter) (*ingest.IngestJob, error) {
	db := dbForContext(ctx, q.DB)

	var users []uint
	err := q.due(ctx, filter).
		Model(&ingest.IngestJob{}).
		Select("user_id").
		Group("user_id").
		Order(orderBy(`(
			SELECT MAX(served.updated_at) FROM ingest_jobs served
			WHERE served.user_id = ingest_jobs.user_id AND served.status <> ?
		) NULLS FIRST, MIN(id)`, ingest.IngestJobPending)).
		Pluck("user_id", &users).Error
	if err != nil {
		return nil, err
	}

	for _, user := range users {
		if err := db.SavePoint("claim").Error; err != nil {
			return nil, err
		}
		free, err := q.takeUserSlot(db, user, filter.PerUser)
		if err != nil {
			return nil, err
		}
		if !free {
			continue
		}
		job, err := q.claim(q.due(ctx, filter).Where("user_id = ?", user).Order("run_at, id"))
		if err != nil || job != nil {
			return job, err
		}
		if err := db.RollbackTo("claim").Error; err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// takeUserSlot holds one of the user's slots until the transaction ends.
func (q *IngestQueue) takeUserSlot(db *gorm.DB, user uint, slots int) (bool, error) {
	for slot := range slots {
		var free bool
		err := db.Raw("SELECT pg_try_advisory_xact_lock(?, ?)", userLockKey+slot, int32(user)).Scan(&free).Error
		if err != nil || free {
			return free, err
		}
	}
	return false, nil
}

func (q *IngestQueue) due(ctx context.Context, filter repositories.JobFilter) *gorm.DB {
	query := dbForContext(ctx, q.DB).Where("status = ? AND run_at <= ?", ingest.IngestJobPending, time.Now())
	if len(filter.Only) > 0 {
		query = query.Where("provider IN ?", filter.Only)
	}
	if len(filter.Except) > 0 {
		query = query.Where("provider NOT IN ?", filter.Except)
	}
	return query
}

func (q *IngestQueue) claim(query *gorm.DB) (*ingest.IngestJob, error) {
	var jobs []ingest.IngestJob
	err := query.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Limit(1).Find(&jobs).Error
	if err != nil || len(jobs) == 0 {
		return nil, err
	}
	return &jobs[0], nil
}

func (q *IngestQueue) Save(ctx context.Context, job *ingest.IngestJob) error {
	return dbForContext(ctx, q.DB).Omit("User", "Batch").Save(job).Error
}

func (q *IngestQueue) Get(ctx context.Context, ids []uint) ([]ingest.IngestJob, error) {
	var jobs []ingest.IngestJob
	err := dbForContext(ctx, q.DB).Where("id IN ?", ids).Order("id").Find(&jobs).Error
	return jobs, err
}

func (q *IngestQueue) CountUnfinished(ctx context.Context) (int64, error) {
	var count int64
	err := dbForContext(ctx, q.DB).
		Model(&ingest.IngestJob{}).
		Where("status = ?", ingest.IngestJobPending).
		Count(&count).Error
	return count, err
}

func (q *IngestQueue) BatchProgress(ctx context.Context, batchID uint) (repositories.BatchProgress, error) {
	var rows []struct {
		Status ingest.IngestJobStatus
		Count  int
	}
	err := dbForContext(ctx, q.DB).
		Model(&ingest.IngestJob{}).
		Select("status, COUNT(*) AS count").
		Where("batch_id = ?", batchID).
		Group("status").
		Scan(&rows).Error
	if err != nil {
		return repositories.BatchProgress{}, err
	}

	var progress repositories.BatchProgress
	for _, row := range rows {
		switch row.Status {
		case ingest.IngestJobDone:
			progress.Done = row.Count
		case ingest.IngestJobFailed:
			progress.Failed = row.Count
		case ingest.IngestJobPending:
			progress.Pending = row.Count
		}
	}
	return progress, nil
}

func (q *IngestQueue) FailedNames(ctx context.Context, batchID uint) ([]string, error) {
	var names []string
	err := dbForContext(ctx, q.DB).
		Model(&ingest.IngestJob{}).
		Where("batch_id = ? AND status = ?", batchID, ingest.IngestJobFailed).
		Order("id").
		Pluck("display_name", &names).Error
	return names, err
}

func (q *IngestQueue) PendingRefs(ctx context.Context, userID uint, providerName provider.ProviderName) ([]string, error) {
	var refs []string
	err := dbForContext(ctx, q.DB).
		Model(&ingest.IngestJob{}).
		Where("user_id = ? AND provider = ? AND status = ?", userID, providerName, ingest.IngestJobPending).
		Pluck("track_ref", &refs).Error
	return refs, err
}
