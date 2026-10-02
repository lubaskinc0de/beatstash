// SharedTrack: a Track of the Shared Library with its Shares, oldest first,
// and how a User shares, unshares or takes it. A Share's SourceTrackID is
// nil once its source is gone. InTop is false if somebody else shared the
// track first.

package sharing

import (
	"errors"
	"slices"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

// SharedTrack is an aggregate root: a Track in the Shared Library and its
// Shares. Several Users can share the same track. It stays shared while at
// least one Share is left. The oldest Share is the author's.
type SharedTrack struct {
	Track  *library.Track
	Shares []Share
}

// Share is an entity inside SharedTrack: one User's share of the track.
type Share struct {
	ID uint `gorm:"primaryKey"`

	TrackID uint          `gorm:"not null;index"`
	Track   library.Track `gorm:"constraint:OnDelete:CASCADE;"`

	SourceTrackID *uint          `gorm:"uniqueIndex"`
	SourceTrack   *library.Track `gorm:"constraint:OnDelete:SET NULL;"`

	UserID uint        `gorm:"not null;index"`
	User   access.User `gorm:"constraint:OnDelete:CASCADE;"`

	InTop bool `gorm:"not null"`

	CreatedAt time.Time `gorm:"not null"`
}

var ErrNotShared = errors.New("track is not in the Shared Library")

// NewSharedTrack is the first Share of a track: its file was copied to the
// Shared Library, and the Share counts in the Top.
func NewSharedTrack(copied *library.Track, sharer *access.User, source *library.Track, at time.Time) *SharedTrack {
	shared := &SharedTrack{Track: copied}
	shared.add(sharer, source, true, at)
	return shared
}

// ShareBy adds a Share of a track that is already shared. There is no
// second copy and it does not count in the Top, but it keeps the track
// shared if the author unshares.
func (s *SharedTrack) ShareBy(sharer *access.User, source *library.Track, at time.Time) *Share {
	return s.add(sharer, source, false, at)
}

func (s *SharedTrack) add(sharer *access.User, source *library.Track, inTop bool, at time.Time) *Share {
	s.Shares = append(s.Shares, Share{
		TrackID:       s.Track.ID,
		SourceTrackID: &source.ID,
		UserID:        sharer.ID,
		User:          *sharer,
		InTop:         inTop,
		CreatedAt:     at,
	})
	return &s.Shares[len(s.Shares)-1]
}

// Unshare removes the Share of the source Track. gone is true when no
// Shares are left and the Track must leave the Shared Library.
func (s *SharedTrack) Unshare(source *library.Track) (gone bool) {
	s.Shares = slices.DeleteFunc(s.Shares, func(share Share) bool {
		return share.SourceTrackID != nil && *share.SourceTrackID == source.ID
	})
	return len(s.Shares) == 0
}

func (s *SharedTrack) Author() *Share {
	if len(s.Shares) == 0 {
		return nil
	}
	return &s.Shares[0]
}

// TakeBy creates a Take credited to the current author.
func (s *SharedTrack) TakeBy(taker *access.User, copied *library.Track, at time.Time) *Take {
	take := &Take{UserID: taker.ID, TrackID: &copied.ID, CreatedAt: at}
	if author := s.Author(); author != nil {
		take.AuthorID = &author.UserID
	}
	return take
}
