package repositories

import (
	"context"

	"github.com/lubaskinc0de/beatstash/internal/domain/library"
)

type ListenLinks interface {
	// OfTrack returns nil if the user has no link to the Track.
	OfTrack(ctx context.Context, userID, trackID uint) (*library.ListenLink, error)
	// OfAlbum returns nil if the user has no link to the Album.
	OfAlbum(ctx context.Context, userID uint, album library.AlbumKey) (*library.ListenLink, error)
	// Save replaces the user's link to the same Track or Album.
	Save(ctx context.Context, link *library.ListenLink) error
}
