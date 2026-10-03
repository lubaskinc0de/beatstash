package database

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/lubaskinc0de/beatstash/internal/application/common/repositories"
	"github.com/lubaskinc0de/beatstash/internal/domain/library"
)

type LibraryRepository struct {
	DB *gorm.DB
}

func (r *LibraryRepository) Ensure(ctx context.Context, lib *library.Library) error {
	db := dbForContext(ctx, r.DB)
	err := db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "dir"}}, DoNothing: true}).
		Create(lib).Error
	if err != nil {
		return err
	}
	return db.Where("dir = ?", lib.Dir).First(lib).Error
}

func (r *LibraryRepository) Shared(ctx context.Context) (*library.Library, error) {
	return r.first(ctx, "kind = ?", library.LibraryShared)
}

func (r *LibraryRepository) Personal(ctx context.Context, userID uint) (*library.Library, error) {
	return r.first(ctx, "owner_id = ?", userID)
}

func (r *LibraryRepository) All(ctx context.Context) ([]library.Library, error) {
	var libraries []library.Library
	err := dbForContext(ctx, r.DB).Order("id").Find(&libraries).Error
	return libraries, err
}

func (r *LibraryRepository) Attached(ctx context.Context) ([]library.Library, error) {
	var libraries []library.Library
	err := dbForContext(ctx, r.DB).Where("kind = ?", library.LibraryAttached).Order("id").Find(&libraries).Error
	return libraries, err
}

func (r *LibraryRepository) Delete(ctx context.Context, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	db := dbForContext(ctx, r.DB)
	if err := db.Where("library_id IN ?", ids).Delete(&library.Track{}).Error; err != nil {
		return err
	}
	return db.Delete(&library.Library{}, ids).Error
}

func (r *LibraryRepository) Save(ctx context.Context, lib *library.Library) error {
	return dbForContext(ctx, r.DB).Save(lib).Error
}

func (r *LibraryRepository) first(ctx context.Context, query string, args ...any) (*library.Library, error) {
	return first[library.Library](dbForContext(ctx, r.DB).Where(query, args...), repositories.ErrLibraryNotFound)
}

func (r *LibraryRepository) Get(ctx context.Context, id uint) (*library.Library, error) {
	return r.first(ctx, "id = ?", id)
}
