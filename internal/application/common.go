package application

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type UserContextKey struct{}

type TrackRepository interface {
	FindSource(ctx context.Context, provider domain.ProviderName, ref string) (*domain.TrackSource, error)
	FindDuplicate(ctx context.Context, m domain.Metadata, durationMs, toleranceMs int) (*domain.Track, error)
	SaveTrack(ctx context.Context, track *domain.Track) error
	SaveSource(ctx context.Context, source *domain.TrackSource) error
	FindTelegramFile(ctx context.Context, m domain.Metadata) (*domain.TelegramFile, error)
}

type UploadRepository interface {
	Save(ctx context.Context, upload *domain.Upload) error
}

type LibraryLock interface {
	// Lock serializes Library writes until ctx's transaction ends.
	Lock(ctx context.Context) error
}

type IngestQueue interface {
	Enqueue(ctx context.Context, job *domain.IngestJob) error
	ClaimNext(ctx context.Context) (*domain.IngestJob, error)
	Save(ctx context.Context, job *domain.IngestJob) error
	CountUnfinished(ctx context.Context) (int64, error)
}

type TxManager interface {
	WithinTx(ctx context.Context, fn func(context.Context) error) error
}

func UserFromContext(ctx context.Context) (*domain.User, bool) {
	user, ok := ctx.Value(UserContextKey{}).(*domain.User)
	return user, ok
}
