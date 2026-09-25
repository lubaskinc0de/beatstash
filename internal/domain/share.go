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

// ShareCopy: the first to share the track counts in the Top.
func ShareCopy(userID uint, source, copied *Track, at time.Time) *Share {
	return &Share{TrackID: copied.ID, SourceTrackID: source.ID, UserID: userID, InTop: true, CreatedAt: at}
}

// ShareDuplicate: no second copy and no Top, but the track stays shared
// after its author unshares.
func ShareDuplicate(userID uint, source, duplicate *Track, at time.Time) *Share {
	return &Share{TrackID: duplicate.ID, SourceTrackID: source.ID, UserID: userID, CreatedAt: at}
}

// Author expects Shares oldest first.
func Author(shares []Share) *Share {
	if len(shares) == 0 {
		return nil
	}
	return &shares[0]
}

func NewTake(userID uint, copied *Track, shares []Share, at time.Time) *Take {
	take := &Take{UserID: userID, TrackID: copied.ID, CreatedAt: at}
	if author := Author(shares); author != nil {
		take.AuthorID = &author.UserID
	}
	return take
}
