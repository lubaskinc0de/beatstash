package domain

import "time"

type ProviderName string

const ProviderTelegram ProviderName = "telegram"

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

	UserID uint `gorm:"not null;index:idx_ingest_job_served,priority:1"`
	User   User `gorm:"constraint:OnDelete:CASCADE;"`

	Provider    ProviderName `gorm:"not null"`
	TrackRef    string       `gorm:"not null"`
	Payload     string       `gorm:"type:jsonb;not null;default:'{}'"`
	DisplayName string

	BatchID *uint       `gorm:"index"`
	Batch   IngestBatch `gorm:"constraint:OnDelete:CASCADE;"`

	// ChatID is zero for a job nobody waits for, e.g. one of a Sync.
	ChatID    int64 `gorm:"not null"`
	MessageID int   `gorm:"not null"`

	Status    IngestJobStatus `gorm:"not null;index:idx_ingest_job_due;index:idx_ingest_job_served,priority:2"`
	Attempts  int             `gorm:"not null"`
	RunAt     time.Time       `gorm:"not null;index:idx_ingest_job_due"`
	LastError string

	CreatedAt time.Time
	UpdatedAt time.Time `gorm:"index:idx_ingest_job_served,priority:3"`
}

func (j *IngestJob) Ref() TrackRef {
	return TrackRef{Provider: j.Provider, ID: j.TrackRef, Payload: j.Payload}
}
