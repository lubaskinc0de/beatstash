package database

import (
	"fmt"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"

	"github.com/lubaskinc0de/beatstash/internal/domain/access"
	"github.com/lubaskinc0de/beatstash/internal/domain/library"
	tgbot "github.com/lubaskinc0de/beatstash/internal/infra/telegram/bot"
	"github.com/lubaskinc0de/beatstash/internal/infra/telegram/poller"
	tgprovider "github.com/lubaskinc0de/beatstash/internal/infra/telegram/provider"
	"github.com/lubaskinc0de/beatstash/internal/infra/telegram/trackfile"
	"github.com/lubaskinc0de/beatstash/internal/infra/telegram/window"
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
	{
		ID: "0012_quotas",
		Migrate: func(tx *gorm.DB) error {
			if err := tx.AutoMigrate(&access.User{}, &library.Library{}, &library.Track{}, &library.QuotaSettings{}); err != nil {
				return err
			}
			// Sync looks up the latest job of each ref of a user.
			return tx.Exec(`CREATE INDEX IF NOT EXISTS idx_ingest_job_ref
				ON ingest_jobs (user_id, provider, track_ref, id DESC)`).Error
		},
	},
	{
		ID: "0013_file_version",
		Migrate: func(tx *gorm.DB) error {
			return tx.Exec(`
				ALTER TABLE tracks ADD COLUMN IF NOT EXISTS file_version bigint NOT NULL DEFAULT 0;
				ALTER TABLE ingest_jobs ADD COLUMN IF NOT EXISTS file_version bigint NOT NULL DEFAULT 0;
				ALTER TABLE telegram_files ADD COLUMN IF NOT EXISTS file_version bigint NOT NULL DEFAULT 0`).Error
		},
	},
	{
		// Tracks there are get the file's state on the first Reconciliation.
		ID: "0014_track_file_state",
		Migrate: func(tx *gorm.DB) error {
			return tx.Exec(`
				ALTER TABLE tracks ADD COLUMN IF NOT EXISTS file_mod_time timestamptz NOT NULL DEFAULT '0001-01-01 00:00:00+00';
				ALTER TABLE tracks ADD COLUMN IF NOT EXISTS file_inode bigint NOT NULL DEFAULT 0`).Error
		},
	},
	{
		ID: "0015_takes_outlive_tracks",
		Migrate: func(tx *gorm.DB) error {
			return tx.Exec(`
				ALTER TABLE takes ALTER COLUMN track_id DROP NOT NULL;
				ALTER TABLE takes DROP CONSTRAINT IF EXISTS fk_takes_track;
				ALTER TABLE takes ADD CONSTRAINT fk_takes_track
					FOREIGN KEY (track_id) REFERENCES tracks(id) ON DELETE SET NULL;
				CREATE INDEX IF NOT EXISTS idx_takes_track_id ON takes (track_id)`).Error
		},
	},
	{
		ID: "0016_library_search",
		Migrate: func(tx *gorm.DB) error {
			if err := tx.Exec(`
				ALTER TABLE tracks ADD COLUMN IF NOT EXISTS follows_song boolean NOT NULL DEFAULT false;
				CREATE EXTENSION IF NOT EXISTS pg_trgm;
				CREATE INDEX IF NOT EXISTS idx_track_recording ON tracks (lower(artist), lower(title))`).Error; err != nil {
				return err
			}
			for _, column := range []string{"artist", "album_artist", "album", "title"} {
				err := tx.Exec(fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_track_search_%s ON tracks USING gin (%s gin_trgm_ops)`,
					column, searchForm(column))).Error
				if err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		ID: "0017_listen_links",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&library.ListenLink{})
		},
	},
	{
		ID: "0018_telegram_share_query",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&tgbot.User{})
		},
	},
	{
		// A new database got the column under its new name in 0001, and the
		// old one in 0016.
		ID: "0019_track_in_attached_library",
		Migrate: func(tx *gorm.DB) error {
			if tx.Migrator().HasColumn(&library.Track{}, "in_attached_library") {
				return tx.Migrator().DropColumn(&library.Track{}, "follows_song")
			}
			return tx.Migrator().RenameColumn(&library.Track{}, "follows_song", "in_attached_library")
		},
	},
	{
		ID: "0020_listen_links_album_case",
		Migrate: func(tx *gorm.DB) error {
			if err := tx.AutoMigrate(&library.ListenLink{}); err != nil {
				return err
			}
			return tx.Exec(`
				DROP INDEX IF EXISTS idx_listen_link_subject;
				CREATE UNIQUE INDEX IF NOT EXISTS idx_listen_link_subject
					ON listen_links (user_id, track_id, library_id, lower(album_artist), lower(album))`).Error
		},
	},
}

func Migrate(db *gorm.DB) error {
	options := *gormigrate.DefaultOptions
	options.UseTransaction = true
	return gormigrate.New(db, &options, migrations).Migrate()
}
