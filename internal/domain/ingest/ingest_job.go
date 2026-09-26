package ingest

import (
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
)

// IngestJob is an aggregate root: a state machine that goes from pending to
// done or failed. It stores no chat or message: a Channel reads the result.
type IngestJob struct {
	ID uint `gorm:"primaryKey"`

	UserID uint        `gorm:"not null;index:idx_ingest_job_served,priority:1"`
	User   access.User `gorm:"constraint:OnDelete:CASCADE;"`

	Provider    provider.ProviderName `gorm:"not null"`
	TrackRef    string                `gorm:"not null"`
	Payload     string                `gorm:"type:jsonb;not null;default:'{}'"`
	DisplayName string

	BatchID *uint       `gorm:"index"`
	Batch   IngestBatch `gorm:"constraint:OnDelete:CASCADE;"`

	Status    IngestJobStatus `gorm:"not null;index:idx_ingest_job_due;index:idx_ingest_job_served,priority:2"`
	Attempts  int             `gorm:"not null"`
	RunAt     time.Time       `gorm:"not null;index:idx_ingest_job_due"`
	LastError string

	// Outcome and TrackID are set when the job is done. TrackID is the
	// Track that was stored or already existed. FailureReason is set when
	// the job failed.
	Outcome       library.Outcome
	TrackID       *uint
	FailureReason FailureReason

	CreatedAt time.Time
	UpdatedAt time.Time `gorm:"index:idx_ingest_job_served,priority:3"`
}

// IngestJobStatus is a value object.
type IngestJobStatus string

const (
	IngestJobPending IngestJobStatus = "pending"
	IngestJobDone    IngestJobStatus = "done"
	IngestJobFailed  IngestJobStatus = "failed"
)

func NewJob(userID uint, ref provider.TrackRef, displayName string, now time.Time) *IngestJob {
	payload := ref.Payload
	if payload == "" {
		payload = "{}"
	}
	return &IngestJob{
		UserID:      userID,
		Provider:    ref.Provider,
		TrackRef:    ref.ID,
		Payload:     payload,
		DisplayName: displayName,
		Status:      IngestJobPending,
		RunAt:       now,
	}
}

func (j *IngestJob) Ref() provider.TrackRef {
	return provider.TrackRef{Provider: j.Provider, ID: j.TrackRef, Payload: j.Payload}
}

func (j *IngestJob) Finished() bool {
	return j.Status != IngestJobPending
}

func (j *IngestJob) Done() bool {
	return j.Status == IngestJobDone
}

func (j *IngestJob) Failed() bool {
	return j.Status == IngestJobFailed
}

func (j *IngestJob) Succeed(outcome library.Outcome, trackID uint) {
	j.Attempts++
	j.Status = IngestJobDone
	j.Outcome = outcome
	j.TrackID = &trackID
	j.LastError = ""
}

// Fail records a failed attempt. The job is retried later, unless the
// error is permanent or the policy has no retries left. final is true
// when the job will not be retried.
func (j *IngestJob) Fail(reason FailureReason, err error, permanent bool, policy RetryPolicy, now time.Time) (final bool) {
	j.Attempts++
	j.LastError = err.Error()

	delay, ok := policy.Delay(j.Attempts)
	if permanent || !ok {
		j.Status = IngestJobFailed
		j.FailureReason = reason
		return true
	}
	j.RunAt = now.Add(delay)
	return false
}
