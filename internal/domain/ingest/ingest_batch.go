// IngestBatch: the Ingest Jobs of one Import, which the User follows as a
// whole.

package ingest

import (
	"time"

	"github.com/lubaskinc0de/beatstash/internal/domain/access"
	"github.com/lubaskinc0de/beatstash/internal/domain/provider"
)

// IngestBatch is an aggregate root: a group of IngestJobs started by one Import.
type IngestBatch struct {
	ID uint `gorm:"primaryKey"`

	UserID uint        `gorm:"not null;index"`
	User   access.User `gorm:"constraint:OnDelete:CASCADE;"`

	Provider provider.ProviderName `gorm:"not null"`
	Kind     IngestBatchKind       `gorm:"not null"`
	Total    int                   `gorm:"not null"`

	CreatedAt  time.Time
	FinishedAt *time.Time `gorm:"index"`
}

// IngestBatchKind is a value object: what started the batch.
type IngestBatchKind string

const (
	IngestBatchImport IngestBatchKind = "import"
)

func (b *IngestBatch) Finished() bool {
	return b.FinishedAt != nil
}

// Finish returns false if the batch was finished already.
func (b *IngestBatch) Finish(at time.Time) bool {
	if b.Finished() {
		return false
	}
	b.FinishedAt = &at
	return true
}

// NewJob creates a job in this batch. Save the batch first: the job needs
// its ID.
func (b *IngestBatch) NewJob(ref provider.TrackRef, displayName string, now time.Time) *IngestJob {
	job := NewJob(b.UserID, ref, displayName, now)
	job.BatchID = &b.ID
	return job
}
