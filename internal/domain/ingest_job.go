package domain

import "time"

type ProviderName string

// TrackRef identifies a track inside a Provider. Payload carries whatever
// else the Provider needs to fetch it, as the Provider encoded it.
type TrackRef struct {
	Provider ProviderName
	ID       string
	Payload  string
}

type IngestJobStatus string

const (
	IngestJobPending IngestJobStatus = "pending"
	IngestJobDone    IngestJobStatus = "done"
	IngestJobFailed  IngestJobStatus = "failed"
)

type IngestJob struct {
	ID uint `gorm:"primaryKey"`

	UserID uint `gorm:"not null"`
	User   User `gorm:"constraint:OnDelete:CASCADE;"`

	Provider ProviderName `gorm:"not null"`
	TrackRef string       `gorm:"not null"`
	Payload  string       `gorm:"type:jsonb;not null;default:'{}'"`

	ChatID    int64 `gorm:"not null"`
	MessageID int   `gorm:"not null"`

	Status    IngestJobStatus `gorm:"not null;index:idx_ingest_job_due"`
	Attempts  int             `gorm:"not null"`
	RunAt     time.Time       `gorm:"not null;index:idx_ingest_job_due"`
	LastError string

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (j *IngestJob) Ref() TrackRef {
	return TrackRef{Provider: j.Provider, ID: j.TrackRef, Payload: j.Payload}
}
