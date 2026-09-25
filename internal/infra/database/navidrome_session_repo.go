package database

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/navidrome"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type NavidromeSessionRepository struct {
	DB *gorm.DB
}

func (r *NavidromeSessionRepository) GetToken(ctx context.Context, username string) (string, error) {
	q := dbForContext(ctx, r.DB).Where("username = ?", username)
	session, err := first[domain.NavidromeSession](q, navidrome.ErrSessionNotFound)
	if err != nil {
		return "", err
	}
	return session.Token, nil
}

func (r *NavidromeSessionRepository) SaveToken(ctx context.Context, username string, token string) error {
	session := domain.NavidromeSession{
		Username: username,
		Token:    token,
	}

	return dbForContext(ctx, r.DB).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "username"}},
			DoUpdates: clause.AssignmentColumns([]string{"token", "updated_at"}),
		}).
		Create(&session).Error
}
