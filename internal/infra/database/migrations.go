package database

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"

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
}

func Migrate(db *gorm.DB) error {
	options := *gormigrate.DefaultOptions
	options.UseTransaction = true
	return gormigrate.New(db, &options, migrations).Migrate()
}
