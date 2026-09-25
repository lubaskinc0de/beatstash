package repositories

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type Libraries interface {
	// Ensure inserts the library unless one with its Dir exists, then loads the stored one into it.
	Ensure(ctx context.Context, library *domain.Library) error
	Shared(ctx context.Context) (*domain.Library, error)
	Personal(ctx context.Context, userID uint) (*domain.Library, error)
	Get(ctx context.Context, id uint) (*domain.Library, error)
	All(ctx context.Context) ([]domain.Library, error)
	SetNavidromeID(ctx context.Context, id uint, navidromeID int) error
}

var ErrLibraryNotFound = errors.New("library not found")

type LibraryLock interface {
	// Lock serializes writes to the libraries until ctx's transaction ends.
	Lock(ctx context.Context, libraryIDs ...uint) error
}
