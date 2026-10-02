// ListenLink: a User keeps one per Track or Album while it lasts. TrackID
// is zero for an Album, and the album fields are empty for a Track. A
// Track's link serves it only while the Track is still the song it leads
// to, SongID. An Album is the Tracks of one Library with the same
// AlbumArtist and Album; it exists only through them.

package library

import "time"

// ListenLink is an aggregate root.
type ListenLink struct {
	ID          uint   `gorm:"primaryKey"`
	UserID      uint   `gorm:"not null"`
	TrackID     uint   `gorm:"not null"`
	LibraryID   uint   `gorm:"not null"`
	AlbumArtist string `gorm:"not null"`
	Album       string `gorm:"not null"`
	SongID      string `gorm:"not null;default:''"`
	URL         string `gorm:"not null"`
	ExpiresAt   time.Time
	CreatedAt   time.Time
}

// AlbumKey is a value object.
type AlbumKey struct {
	LibraryID   uint
	AlbumArtist string
	Album       string
}

func NewTrackListenLink(userID uint, track *Track, url string, expires time.Time) *ListenLink {
	return &ListenLink{
		UserID: userID, TrackID: track.ID, LibraryID: track.LibraryID, SongID: track.SongID, URL: url, ExpiresAt: expires,
	}
}

func NewAlbumListenLink(userID uint, album AlbumKey, url string, expires time.Time) *ListenLink {
	return &ListenLink{
		UserID: userID, LibraryID: album.LibraryID, AlbumArtist: album.AlbumArtist, Album: album.Album, URL: url, ExpiresAt: expires,
	}
}

func (l *ListenLink) Valid(now time.Time) bool {
	return now.Before(l.ExpiresAt)
}

func (l *ListenLink) ServesTrack(track *Track, now time.Time) bool {
	return l.Valid(now) && l.SongID == track.SongID
}
