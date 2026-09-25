package bot

import (
	"context"
	"errors"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/application"
)

func sender(update *models.Update) *models.User {
	switch {
	case update.Message != nil:
		return update.Message.From
	case update.InlineQuery != nil:
		return update.InlineQuery.From
	case update.CallbackQuery != nil:
		return &update.CallbackQuery.From
	}

	return nil
}

func userMiddleware(authenticate *application.Authenticate) bot.Middleware {
	return func(next bot.HandlerFunc) bot.HandlerFunc {
		return func(
			ctx context.Context,
			b *bot.Bot,
			update *models.Update,
		) {
			from := sender(update)
			if from == nil {
				slog.Info("access_denied", "reason", "unknown_update_type")
				return
			}

			profile := application.TelegramProfile{ID: uint64(from.ID), Username: from.Username}
			user, err := authenticate.Execute(ctx, profile)
			if errors.Is(err, application.ErrUserNotFound) {
				if isStart(update) {
					next(ctx, b, update)
					return
				}
				slog.Info("access_denied", "uid", profile.ID)
				return
			}
			if err != nil {
				slog.Error("authenticate", "error", err)
				return
			}

			next(application.WithUser(ctx, user), b, update)
		}
	}
}

// isStart lets a stranger accept an invite or learn what the bot is.
func isStart(update *models.Update) bool {
	name, _, ok := command(update)
	return ok && name == "start"
}
