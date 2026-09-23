package database

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/application"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type InviteRepository struct {
	db *gorm.DB
}

func NewInviteRepository(db *gorm.DB) *InviteRepository {
	return &InviteRepository{db: db}
}

func (r *InviteRepository) Save(ctx context.Context, invite *domain.Invite) error {
	return dbForContext(ctx, r.db).Save(invite).Error
}

// GetForUpdate locks the invite until ctx's transaction ends.
func (r *InviteRepository) GetForUpdate(ctx context.Context, code string) (*domain.Invite, error) {
	var invite domain.Invite

	err := dbForContext(ctx, r.db).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("code = ?", code).
		First(&invite).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, application.ErrInviteInvalid
	}
	if err != nil {
		return nil, err
	}
	return &invite, nil
}
