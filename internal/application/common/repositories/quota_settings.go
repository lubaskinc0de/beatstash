package repositories

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

type QuotaSettings interface {
	// Get returns new settings, all following the config, if none are saved.
	Get(ctx context.Context) (*library.QuotaSettings, error)
	Save(ctx context.Context, settings *library.QuotaSettings) error
}
