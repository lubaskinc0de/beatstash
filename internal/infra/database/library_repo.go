package database

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/lubaskinc0de/navidrome-tg/internal/application"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type LibraryRepository struct {
	db *gorm.DB
}

func NewLibraryRepository(db *gorm.DB) *LibraryRepository {
	return &LibraryRepository{db: db}
}

func (r *LibraryRepository) Ensure(ctx context.Context, library *domain.Library) error {
	db := dbForContext(ctx, r.db)
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
	err := dbForContext(ctx, r.db).Order("id").Find(&libraries).Error
	return libraries, err
}

func (r *LibraryRepository) SetNavidromeID(ctx context.Context, id uint, navidromeID int) error {
	return dbForContext(ctx, r.db).
		Model(&domain.Library{}).
		Where("id = ?", id).
		Update("navidrome_id", navidromeID).Error
}

func (r *LibraryRepository) first(ctx context.Context, query string, args ...any) (*domain.Library, error) {
	var library domain.Library
	err := dbForContext(ctx, r.db).Where(query, args...).First(&library).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, application.ErrLibraryNotFound
	}
	if err != nil {
		return nil, err
	}
	return &library, nil
}
