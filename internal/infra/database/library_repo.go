package database

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/lubaskinc0de/navidrome-tg/internal/application"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type LibraryRepository struct {
	DB *gorm.DB
}

func (r *LibraryRepository) Ensure(ctx context.Context, library *domain.Library) error {
	db := dbForContext(ctx, r.DB)
	err := db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "dir"}}, DoNothing: true}).
		Create(library).Error
	if err != nil {
		return err
	}
	return db.Where("dir = ?", library.Dir).First(library).Error
}

func (r *LibraryRepository) Shared(ctx context.Context) (*domain.Library, error) {
	return r.first(ctx, "kind = ?", domain.LibraryShared)
}

func (r *LibraryRepository) Personal(ctx context.Context, userID uint) (*domain.Library, error) {
	return r.first(ctx, "owner_id = ?", userID)
}

func (r *LibraryRepository) All(ctx context.Context) ([]domain.Library, error) {
	var libraries []domain.Library
	err := dbForContext(ctx, r.DB).Order("id").Find(&libraries).Error
	return libraries, err
}

func (r *LibraryRepository) SetNavidromeID(ctx context.Context, id uint, navidromeID int) error {
	return dbForContext(ctx, r.DB).
		Model(&domain.Library{}).
		Where("id = ?", id).
		Update("navidrome_id", navidromeID).Error
}

func (r *LibraryRepository) first(ctx context.Context, query string, args ...any) (*domain.Library, error) {
	return first[domain.Library](dbForContext(ctx, r.DB).Where(query, args...), application.ErrLibraryNotFound)
}
