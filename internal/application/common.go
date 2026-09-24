package application

import (
	"context"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type UserContextKey struct{}

type TrackRepository interface {
	FindSource(ctx context.Context, libraryID uint, provider domain.ProviderName, ref string) (*domain.TrackSource, error)
	FindDuplicate(ctx context.Context, libraryID uint, m domain.Metadata, durationMs int) (*domain.Track, error)
	// FindByMetadata matches artist and title, preferring the same album.
	FindByMetadata(ctx context.Context, libraryID uint, m domain.Metadata) (*domain.Track, error)
	SaveTrack(ctx context.Context, track *domain.Track) error
	SaveSource(ctx context.Context, source *domain.TrackSource) error
	FindTelegramFile(ctx context.Context, libraryIDs []uint, m domain.Metadata) (*domain.TelegramFile, error)
	Get(ctx context.Context, id uint) (*domain.Track, error)
	Sources(ctx context.Context, trackID uint) ([]domain.TrackSource, error)
	// Album lists the library's Tracks of the album, by track number.
	Album(ctx context.Context, libraryID uint, albumArtist, album string) ([]domain.Track, error)
	Delete(ctx context.Context, id uint) error
	// TelegramFile prefers files sendable as audio; ErrTrackNotFound if the Track has none.
	TelegramFile(ctx context.Context, trackID uint) (*domain.TelegramFile, error)
}

type ShareRepository interface {
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

type TakeRepository interface {
	Save(ctx context.Context, take *domain.Take) error
	// TopTaken counts Takes of each author's Shares made since the time, except the author's own.
	TopTaken(ctx context.Context, since time.Time, limit int) ([]TopEntry, error)
}

type TopEntry struct {
	User  domain.User
	Count int
}

type LibraryRepository interface {
	// Ensure inserts the library unless one with its Dir exists, then loads the stored one into it.
	Ensure(ctx context.Context, library *domain.Library) error
	Shared(ctx context.Context) (*domain.Library, error)
	Personal(ctx context.Context, userID uint) (*domain.Library, error)
	All(ctx context.Context) ([]domain.Library, error)
	SetNavidromeID(ctx context.Context, id uint, navidromeID int) error
}

type UploadRepository interface {
	Save(ctx context.Context, upload *domain.Upload) error
}

type LibraryLock interface {
	// Lock serializes writes to the libraries until ctx's transaction ends.
	Lock(ctx context.Context, libraryIDs ...uint) error
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
