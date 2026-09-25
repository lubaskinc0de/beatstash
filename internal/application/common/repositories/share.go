package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type Shares interface {
	Save(ctx context.Context, share *domain.Share) error
	// BySource returns ErrShareNotFound unless the Track is shared.
	BySource(ctx context.Context, sourceTrackID uint) (*domain.Share, error)
	// ForTrack lists the Shares of a Shared Library Track with their Users, oldest first.
	ForTrack(ctx context.Context, trackID uint) ([]domain.Share, error)
	Delete(ctx context.Context, id uint) error
	// Feed lists the Shared Library's Tracks with their authors, newest first.
	Feed(ctx context.Context, limit int) ([]domain.Share, error)
	// TopSharers counts the Shares that go to the Top made since the time.
	TopSharers(ctx context.Context, since time.Time, limit int) ([]TopEntry, error)
}

var ErrShareNotFound = errors.New("share not found")

type TopEntry struct {
	User  domain.User
	Count int
}
