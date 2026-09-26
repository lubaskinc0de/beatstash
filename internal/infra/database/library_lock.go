package database

import (
	"context"
	"slices"

	"gorm.io/gorm"
)

const libraryLockKey = 7_246_101

type LibraryLock struct {
	DB *gorm.DB
}

// Lock takes the libraries in ascending id order, so two transactions
// locking the same pair can never wait on each other.
func (l *LibraryLock) Lock(ctx context.Context, libraryIDs ...uint) error {
	ids := slices.Clone(libraryIDs)
	slices.Sort(ids)
	for _, id := range slices.Compact(ids) {
		err := dbForContext(ctx, l.DB).Exec("SELECT pg_advisory_xact_lock(?, ?)", libraryLockKey, int32(id)).Error //nolint:gosec // G115: lock key only; wraparound merely shares a lock
		if err != nil {
			return err
		}
	}
	return nil
}
