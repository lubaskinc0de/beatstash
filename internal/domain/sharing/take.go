// Take: a copy of a Shared Track that a User took into their Personal
// Library. TrackID is nil once the copy is gone; the Take still counts for
// the Top.

package sharing

import (
	"time"

	"github.com/lubaskinc0de/beatstash/internal/domain/access"
	"github.com/lubaskinc0de/beatstash/internal/domain/library"
)

// Take is an aggregate root: a copy of a SharedTrack in the taker's Personal
// Library. It does not point to the Share, because the Share may be deleted
// before the Top is counted. It outlives its copy too: the Top counts that
// the Take happened.
type Take struct {
	ID uint `gorm:"primaryKey"`

	UserID uint        `gorm:"not null;index"`
	User   access.User `gorm:"constraint:OnDelete:CASCADE;"`

	TrackID *uint          `gorm:"index"`
	Track   *library.Track `gorm:"constraint:OnDelete:SET NULL;"`

	AuthorID *uint `gorm:"index"`

	CreatedAt time.Time `gorm:"not null"`
}
