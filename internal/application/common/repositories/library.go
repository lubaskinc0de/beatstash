package repositories

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/beatstash/internal/domain/library"
)

type Libraries interface {
	// Ensure inserts the library unless one with its Dir exists, then loads the stored one into it.
	Ensure(ctx context.Context, lib *library.Library) error
	Shared(ctx context.Context) (*library.Library, error)
	Personal(ctx context.Context, userID uint) (*library.Library, error)
	Get(ctx context.Context, id uint) (*library.Library, error)
	All(ctx context.Context) ([]library.Library, error)
	Attached(ctx context.Context) ([]library.Library, error)
	Save(ctx context.Context, lib *library.Library) error
	// Delete removes the libraries along with their Tracks.
	Delete(ctx context.Context, ids []uint) error
}

var ErrLibraryNotFound = errors.New("library not found")

type LibraryLock interface {
	// Lock serializes writes to the libraries until ctx's transaction ends.
	Lock(ctx context.Context, libraryIDs ...uint) error
}
