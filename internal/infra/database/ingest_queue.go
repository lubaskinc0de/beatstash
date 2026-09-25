package database

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type IngestQueue struct {
	DB *gorm.DB
}

func (q *IngestQueue) Enqueue(ctx context.Context, job *domain.IngestJob) error {
	return dbForContext(ctx, q.DB).Omit("User").Create(job).Error
}

// ClaimNext keeps the job pending while locked, so a crashed worker's job
// becomes claimable again as soon as its transaction dies.
func (q *IngestQueue) ClaimNext(ctx context.Context) (*domain.IngestJob, error) {
	var jobs []domain.IngestJob

	err := dbForContext(ctx, q.DB).
		Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
		Where("status = ? AND run_at <= ?", domain.IngestJobPending, time.Now()).
		Order("run_at, id").
		Limit(1).
		Find(&jobs).Error
	if err != nil || len(jobs) == 0 {
		return nil, err
	}
	return &jobs[0], nil
}

func (q *IngestQueue) Save(ctx context.Context, job *domain.IngestJob) error {
	return dbForContext(ctx, q.DB).Omit("User").Save(job).Error
}

func (q *IngestQueue) CountUnfinished(ctx context.Context) (int64, error) {
	var count int64
	err := dbForContext(ctx, q.DB).
		Model(&domain.IngestJob{}).
		Where("status = ?", domain.IngestJobPending).
		Count(&count).Error
	return count, err
}
