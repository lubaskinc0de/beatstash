package database

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/store"
)

// migrations change the schema; each runs once, in a transaction, and the
// done ones are kept in the migrations table.
var migrations = []*gormigrate.Migration{
	{
		ID: "0001_initial",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(append(models, store.Models...)...)
		},
	},
	{
		ID: "0002_telegram_window",
		Migrate: func(tx *gorm.DB) error {
			if err := tx.Migrator().DropTable("telegram_dialogs", "telegram_batch_messages"); err != nil {
				return err
			}
			return tx.AutoMigrate(&store.Window{}, &store.User{}, &store.FollowedBatch{})
		},
	},
	{
		ID: "0003_telegram_window_below",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&store.Window{})
		},
	},
	{
		ID: "0004_attached_library",
		Migrate: func(tx *gorm.DB) error {
			if err := tx.AutoMigrate(&library.Track{}); err != nil {
				return err
			}
			return tx.Exec(`
				ALTER TABLE shares ALTER COLUMN source_track_id DROP NOT NULL;
				ALTER TABLE shares DROP CONSTRAINT IF EXISTS fk_shares_source_track;
				ALTER TABLE shares ADD CONSTRAINT fk_shares_source_track
					FOREIGN KEY (source_track_id) REFERENCES tracks(id) ON DELETE SET NULL`).Error
		},
	},
}

func Migrate(db *gorm.DB) error {
	options := *gormigrate.DefaultOptions
	options.UseTransaction = true
	return gormigrate.New(db, &options, migrations).Migrate()
}
