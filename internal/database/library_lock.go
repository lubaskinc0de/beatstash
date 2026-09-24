package database

import (
	"context"
	"slices"

	"gorm.io/gorm"
)

const libraryLockKey = 7_246_101

type LibraryLock struct {
	db *gorm.DB
}

func NewLibraryLock(db *gorm.DB) *LibraryLock {
	return &LibraryLock{db: db}
}

// Lock takes the libraries in ascending id order, so two transactions
// locking the same pair can never wait on each other.
func (l *LibraryLock) Lock(ctx context.Context, libraryIDs ...uint) error {
	ids := slices.Clone(libraryIDs)
	slices.Sort(ids)
	for _, id := range slices.Compact(ids) {
		err := dbForContext(ctx, l.db).Exec("SELECT pg_advisory_xact_lock(?, ?)", libraryLockKey, int32(id)).Error
		if err != nil {
			return err
		}
	}
	return nil
}
