package harness

import (
	"time"

	tgbot "github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/bot"
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

func WithTelegramPollInterval(d time.Duration) Option {
	return func(c *app.Config) { c.TelegramPollInterval = d }
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

// WithAdmins names the Admins in the config instead of the scenario's Admin.
func WithAdmins(users ...User) Option {
	return func(c *app.Config) {
		c.Admins = nil
		for _, user := range users {
			c.Admins = append(c.Admins, tgbot.Identity(user.ID))
		}
	}
}

func WithServiceName(name string) Option {
	return func(c *app.Config) { c.ServiceName = name }
}

func WithTranslations(dir string) Option {
	return func(c *app.Config) { c.TranslationsDir = dir }
}

func WithAttachInterval(d time.Duration) Option {
	return func(c *app.Config) { c.AttachInterval = d }
}
