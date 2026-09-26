package database

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
)

type InviteRepository struct {
	DB *gorm.DB
}

func (r *InviteRepository) Save(ctx context.Context, invite *access.Invite) error {
	return dbForContext(ctx, r.DB).Save(invite).Error
}

// GetForUpdate locks the invite until ctx's transaction ends.
func (r *InviteRepository) GetForUpdate(ctx context.Context, code string) (*access.Invite, error) {
	q := dbForContext(ctx, r.DB).Clauses(clause.Locking{Strength: "UPDATE"}).Where("code = ?", code)
	return first[access.Invite](q, access.ErrInviteInvalid)
}
