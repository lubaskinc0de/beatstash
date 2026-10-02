// IngestJob: one planned Ingest of a Track Ref and how it ended. A done job
// keeps its Outcome, the Track that was stored or already existed, and the
// version of that Track's file the job left. A failed job keeps its
// FailureReason; a RetryPolicy decides when a failed attempt runs again.

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

	Outcome       library.Outcome
	TrackID       *uint
	FileVersion   int `gorm:"not null;default:0"`
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

// FailureReason is a value object: why an IngestJob failed and will not be retried.
type FailureReason string

const (
	ReasonUnsupportedFormat FailureReason = "unsupported_format"
	ReasonCorruptFile       FailureReason = "corrupt_file"
	ReasonFetchFailed       FailureReason = "fetch_failed"
	ReasonNoProviderAccount FailureReason = "no_provider_account"
	ReasonTokenRejected     FailureReason = "token_rejected"
	ReasonQuotaExceeded     FailureReason = "quota_exceeded"
	ReasonInternal          FailureReason = "internal"
)

// RetryPolicy is a value object: how long to wait before each retry of a
// failed IngestJob.
type RetryPolicy struct {
	Delays []time.Duration
}

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

func (j *IngestJob) Succeed(outcome library.Outcome, trackID uint, fileVersion int) {
	j.Attempts++
	j.Status = IngestJobDone
	j.Outcome = outcome
	j.TrackID = &trackID
	j.FileVersion = fileVersion
	j.LastError = ""
}

// Delivered reports whether the job's audio became the Track's file: an
// audio that already existed did not.
func (j *IngestJob) Delivered() bool {
	return j.Done() && j.Outcome != library.AlreadyExists
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

// AwaitsRoom reports that the job failed because the Library had no room
// for the Track: trying again makes sense once the Library has room.
func (r FailureReason) AwaitsRoom() bool {
	return r == ReasonQuotaExceeded
}

// Delay is how long to wait after the given failed attempt. It returns
// false when no retries are left.
func (p RetryPolicy) Delay(attempt int) (time.Duration, bool) {
	if attempt < 1 || attempt > len(p.Delays) {
		return 0, false
	}
	return p.Delays[attempt-1], true
}
