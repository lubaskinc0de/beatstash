package database

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

type ListenLinkRepository struct {
	DB *gorm.DB
}

func (r *ListenLinkRepository) OfTrack(ctx context.Context, userID, trackID uint) (*library.ListenLink, error) {
	return r.find(dbForContext(ctx, r.DB).Where("user_id = ? AND track_id = ?", userID, trackID))
}

func (r *ListenLinkRepository) OfAlbum(ctx context.Context, userID uint, album library.AlbumKey) (*library.ListenLink, error) {
	return r.find(dbForContext(ctx, r.DB).Where(
		"user_id = ? AND track_id = 0 AND library_id = ? AND lower(album_artist) = lower(?) AND lower(album) = lower(?)",
		userID, album.LibraryID, album.AlbumArtist, album.Album,
	))
}

func (r *ListenLinkRepository) find(q *gorm.DB) (*library.ListenLink, error) {
	link, err := first[library.ListenLink](q, errNoLink)
	if errors.Is(err, errNoLink) {
		return nil, nil
	}
	return link, err
}

var errNoLink = errors.New("no listen link")

// Save keeps one link per Album whatever the case of its names.
func (r *ListenLinkRepository) Save(ctx context.Context, link *library.ListenLink) error {
	return dbForContext(ctx, r.DB).Exec(`
		INSERT INTO listen_links (user_id, track_id, library_id, album_artist, album, song_id, url, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, now())
		ON CONFLICT (user_id, track_id, library_id, lower(album_artist), lower(album)) DO UPDATE SET
			album_artist = excluded.album_artist, album = excluded.album, song_id = excluded.song_id,
			url = excluded.url, expires_at = excluded.expires_at, created_at = excluded.created_at`,
		link.UserID, link.TrackID, link.LibraryID, link.AlbumArtist, link.Album, link.SongID, link.URL, link.ExpiresAt,
	).Error
}
