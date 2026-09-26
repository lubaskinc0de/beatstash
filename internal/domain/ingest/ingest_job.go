package ingest

import (
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
)

type IngestJobStatus string

const (
	IngestJobPending IngestJobStatus = "pending"
	IngestJobDone    IngestJobStatus = "done"
	IngestJobFailed  IngestJobStatus = "failed"
)

type FailureReason string

const (
	ReasonUnsupportedFormat FailureReason = "unsupported_format"
	ReasonCorruptFile       FailureReason = "corrupt_file"
	ReasonFetchFailed       FailureReason = "fetch_failed"
	ReasonNoProviderAccount FailureReason = "no_provider_account"
	ReasonTokenRejected     FailureReason = "token_rejected"
	ReasonInternal          FailureReason = "internal"
)

// IngestJob keeps no address to answer to: a Channel reads its outcome.
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

	// Outcome and TrackID, the Track the audio became or turned out to be,
	// are set once the job is done; FailureReason once it failed.
	Outcome       library.Outcome
	TrackID       *uint
	FailureReason FailureReason

	CreatedAt time.Time
	UpdatedAt time.Time `gorm:"index:idx_ingest_job_served,priority:3"`
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
