package ingest

import (
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
)

type IngestBatchKind string

const (
	IngestBatchImport IngestBatchKind = "import"
)

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
