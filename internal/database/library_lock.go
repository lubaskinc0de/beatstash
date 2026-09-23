package database

import (
	"context"

	"gorm.io/gorm"
)

const libraryLockKey = 7_246_101

type LibraryLock struct {
	db *gorm.DB
}

func NewLibraryLock(db *gorm.DB) *LibraryLock {
	return &LibraryLock{db: db}
}

func (l *LibraryLock) Lock(ctx context.Context) error {
	return dbForContext(ctx, l.db).Exec("SELECT pg_advisory_xact_lock(?)", libraryLockKey).Error
}
