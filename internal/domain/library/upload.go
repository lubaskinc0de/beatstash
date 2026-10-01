// Upload: the record that a User sent a Track through one of its Track
// Sources.

package library

import (
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
)

// Upload is an aggregate root: a record that a User sent a Track through a
// TrackSource. It is not part of Track because a Track can have many Uploads
// and no rule ties them to the Track.
type Upload struct {
	ID uint `gorm:"primaryKey"`

	UserID uint        `gorm:"not null;index"`
	User   access.User `gorm:"constraint:OnDelete:CASCADE;"`

	TrackID uint  `gorm:"not null;index"`
	Track   Track `gorm:"constraint:OnDelete:CASCADE;"`

	TrackSourceID uint        `gorm:"not null"`
	TrackSource   TrackSource `gorm:"constraint:OnDelete:CASCADE;"`

	CreatedAt time.Time
}

// NewUpload records that the user sent the Source's Track.
func NewUpload(userID uint, source *TrackSource) *Upload {
	return &Upload{UserID: userID, TrackID: source.TrackID, TrackSourceID: source.ID}
}
