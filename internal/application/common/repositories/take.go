package repositories

import (
	"context"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/sharing"
)

type Takes interface {
	Save(ctx context.Context, take *sharing.Take) error
	// TopTaken counts Takes of each author's Shares made since the time, except the author's own.
	TopTaken(ctx context.Context, since time.Time, limit int) ([]TopEntry, error)
}
