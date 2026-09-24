package domain

import "time"

// Share puts an author's Track into the Shared Library as a Track of its
// own. Several Users may share the same track: it stays shared while any
// of them keeps their Share, and the oldest one is its author.
type Share struct {
	ID uint `gorm:"primaryKey"`

	TrackID uint  `gorm:"not null;index"`
	Track   Track `gorm:"constraint:OnDelete:CASCADE;"`

	SourceTrackID uint  `gorm:"not null;uniqueIndex"`
	SourceTrack   Track `gorm:"constraint:OnDelete:CASCADE;"`

	UserID uint `gorm:"not null;index"`
	User   User `gorm:"constraint:OnDelete:CASCADE;"`

	// InTop is false for a Share of a track somebody had shared first.
	InTop bool `gorm:"not null"`

	CreatedAt time.Time `gorm:"not null"`
}

// Take is a copy of a shared Track in the taker's Personal Library. It keeps
// no link to the Share, which may be gone long before the Top is counted.
type Take struct {
	ID uint `gorm:"primaryKey"`

	UserID uint `gorm:"not null;index"`
	User   User `gorm:"constraint:OnDelete:CASCADE;"`

	TrackID uint  `gorm:"not null"`
	Track   Track `gorm:"constraint:OnDelete:CASCADE;"`

	AuthorID *uint `gorm:"index"`

	CreatedAt time.Time `gorm:"not null"`
}
