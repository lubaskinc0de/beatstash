package database

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

type QuotaSettingsRepository struct {
	DB *gorm.DB
}

func (r *QuotaSettingsRepository) Get(ctx context.Context) (*library.QuotaSettings, error) {
	settings := library.NewQuotaSettings()
	err := dbForContext(ctx, r.DB).Where("id = ?", settings.ID).First(settings).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return library.NewQuotaSettings(), nil
	}
	return settings, err
}

func (r *QuotaSettingsRepository) Save(ctx context.Context, settings *library.QuotaSettings) error {
	return dbForContext(ctx, r.DB).Save(settings).Error
}
