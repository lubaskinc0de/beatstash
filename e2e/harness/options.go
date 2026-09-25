package harness

import (
	"time"

	app "github.com/lubaskinc0de/navidrome-tg/internal/main"
)

func WithAdminContact(contact string) Option {
	return func(c *app.Config) { c.AdminContact = contact }
}

func WithoutWorkers() Option {
	return func(c *app.Config) {
		c.IngestWorkers = 0
		c.ZvukWorkers = 0
	}
}

func WithZvukWorkers(n int) Option {
	return func(c *app.Config) { c.ZvukWorkers = n }
}

func WithZvukPerUser(n int) Option {
	return func(c *app.Config) { c.ZvukPerUser = n }
}

func WithProgressInterval(d time.Duration) Option {
	return func(c *app.Config) { c.ProgressInterval = d }
}

func WithZvukPause(d time.Duration) Option {
	return func(c *app.Config) { c.ZvukPauseMin, c.ZvukPauseMax = d, d }
}

func WithSyncInterval(d time.Duration) Option {
	return func(c *app.Config) { c.SyncInterval = d }
}

func WithStorageChat(chatID int64) Option {
	return func(c *app.Config) { c.StorageChatID = chatID }
}

func WithMaxUpload(bytes int64) Option {
	return func(c *app.Config) { c.MaxPostSize = bytes }
}
