package database

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
	tgbot "github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/bot"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/poller"
	tgprovider "github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/provider"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/trackfile"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/window"
)

var telegramModels = []any{
	&window.Window{},
	&tgbot.User{},
	&poller.JobMessage{},
	&poller.FollowedBatch{},
	&poller.AccountNotice{},
	&trackfile.File{},
	&trackfile.Post{},
	&tgprovider.LocalFile{},
}

// migrations change the schema; each runs once, in a transaction, and the
// done ones are kept in the migrations table.
var migrations = []*gormigrate.Migration{
	{
		ID: "0001_initial",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(append(models, telegramModels...)...)
		},
	},
	{
		ID: "0002_telegram_window",
		Migrate: func(tx *gorm.DB) error {
			if err := tx.Migrator().DropTable("telegram_dialogs", "telegram_batch_messages"); err != nil {
				return err
			}
			return tx.AutoMigrate(&window.Window{}, &tgbot.User{}, &poller.FollowedBatch{})
		},
	},
	{
		ID: "0003_telegram_window_below",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&window.Window{})
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
	{
		ID: "0005_telegram_uploads",
		Migrate: func(tx *gorm.DB) error {
			return tx.Exec(`CREATE TABLE IF NOT EXISTS telegram_uploads (
				track_id bigint PRIMARY KEY, started_at timestamptz NOT NULL)`).Error
		},
	},
	{
		ID: "0006_telegram_job_messages_claim",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&poller.JobMessage{})
		},
	},
	{
		ID: "0007_telegram_followed_batches_claim",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&poller.FollowedBatch{})
		},
	},
	{
		// Users whose token is rejected now hear of it once more.
		ID: "0008_telegram_account_notices",
		Migrate: func(tx *gorm.DB) error {
			if err := tx.Migrator().DropTable("telegram_account_notices"); err != nil {
				return err
			}
			return tx.AutoMigrate(&poller.AccountNotice{})
		},
	},
	{
		// Orphans of deleted Tracks go first: the key would not be made
		// with them.
		ID: "0009_telegram_file_posts",
		Migrate: func(tx *gorm.DB) error {
			if err := tx.Migrator().DropTable("telegram_uploads"); err != nil {
				return err
			}
			if err := tx.AutoMigrate(&trackfile.Post{}); err != nil {
				return err
			}
			return tx.Exec(`
				DELETE FROM telegram_files WHERE NOT EXISTS (SELECT 1 FROM tracks WHERE tracks.id = telegram_files.track_id);
				ALTER TABLE telegram_files DROP CONSTRAINT IF EXISTS fk_telegram_files_track;
				ALTER TABLE telegram_files ADD CONSTRAINT fk_telegram_files_track
					FOREIGN KEY (track_id) REFERENCES tracks(id) ON DELETE CASCADE;
				ALTER TABLE telegram_file_posts DROP CONSTRAINT IF EXISTS fk_telegram_file_posts_track;
				ALTER TABLE telegram_file_posts ADD CONSTRAINT fk_telegram_file_posts_track
					FOREIGN KEY (track_id) REFERENCES tracks(id) ON DELETE CASCADE`).Error
		},
	},
	{
		ID: "0010_telegram_window_lease",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&window.Window{})
		},
	},
	{
		ID: "0011_telegram_local_files",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&tgprovider.LocalFile{})
		},
	},
}

func Migrate(db *gorm.DB) error {
	options := *gormigrate.DefaultOptions
	options.UseTransaction = true
	return gormigrate.New(db, &options, migrations).Migrate()
}
