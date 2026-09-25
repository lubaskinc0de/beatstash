package database

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/navidrome"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type NavidromeSessionRepository struct {
	db *gorm.DB
}

func NewNavidromeSessionRepository(db *gorm.DB) *NavidromeSessionRepository {
	return &NavidromeSessionRepository{
		db: db,
	}
}

func (r *NavidromeSessionRepository) GetToken(ctx context.Context, username string) (string, error) {
	var session domain.NavidromeSession

	err := dbForContext(ctx, r.db).Where("username = ?", username).First(&session).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", navidrome.ErrSessionNotFound
		}
		return "", err
	}
	return session.Token, nil
}

func (r *NavidromeSessionRepository) SaveToken(ctx context.Context, username string, token string) error {
	session := domain.NavidromeSession{
		Username: username,
		Token:    token,
	}

	return dbForContext(ctx, r.db).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "username"}},
			DoUpdates: clause.AssignmentColumns([]string{"token", "updated_at"}),
		}).
		Create(&session).Error
}
