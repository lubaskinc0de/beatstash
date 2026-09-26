package sharing

import (
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

// Take is an aggregate root: a copy of a SharedTrack in the taker's Personal
// Library. It does not point to the Share, because the Share may be deleted
// before the Top is counted.
type Take struct {
	ID uint `gorm:"primaryKey"`

	UserID uint        `gorm:"not null;index"`
	User   access.User `gorm:"constraint:OnDelete:CASCADE;"`

	TrackID uint          `gorm:"not null"`
	Track   library.Track `gorm:"constraint:OnDelete:CASCADE;"`

	AuthorID *uint `gorm:"index"`

	CreatedAt time.Time `gorm:"not null"`
}
