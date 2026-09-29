package repositories

import (
	"context"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/sharing"
)

// SharedTracks keeps the Shared Library's Tracks together with their Shares.
type SharedTracks interface {
	// Get returns sharing.ErrNotShared unless the Track has a Share.
	Get(ctx context.Context, trackID uint) (*sharing.SharedTrack, error)
	// BySource finds what the user's Track is shared as; sharing.ErrNotShared
	// unless it is shared.
	BySource(ctx context.Context, sourceTrackID uint) (*sharing.SharedTrack, error)
	// SharedSources tells which of the user's Tracks are shared.
	SharedSources(ctx context.Context, sourceTrackIDs []uint) (map[uint]bool, error)
	// Save stores the new Shares and drops the ones gone.
	Save(ctx context.Context, shared *sharing.SharedTrack) error
	Delete(ctx context.Context, shared *sharing.SharedTrack) error
	// Feed lists the Shared Library's Tracks with their authors, newest first.
	Feed(ctx context.Context, limit int) ([]sharing.Share, error)
	// TopSharers counts the Shares that go to the Top made since the time.
	TopSharers(ctx context.Context, since time.Time, limit int) ([]TopEntry, error)
}

type TopEntry struct {
	User  access.User
	Count int
}
