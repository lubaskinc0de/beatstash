package telegram

import (
	"context"
	"errors"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/application"
	"github.com/lubaskinc0de/navidrome-tg/internal/database"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

func withUser(ctx context.Context, user *domain.User) context.Context {
	return context.WithValue(ctx, application.UserContextKey{}, user)
}

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

func UserMiddleware(users *database.UserRepository) bot.Middleware {
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
			telegramID := uint64(from.ID)

			user, err := users.GetById(ctx, telegramID)
			if errors.Is(err, application.ErrUserNotFound) {
				if isStart(update) {
					next(ctx, b, update)
					return
				}
				slog.Info("access_denied", "uid", telegramID)
				return
			}
			if err != nil {
				slog.Error("get_user", "error", err)
				return
			}

			// Admins from ADMIN_IDS start without a username, and people rename themselves.
			if from.Username != user.Username {
				if err := users.SetUsername(ctx, user.ID, from.Username); err != nil {
					slog.Error("set_username", "error", err)
				} else {
					user.Username = from.Username
				}
			}

			next(withUser(ctx, user), b, update)
		}
	}
}

// isStart lets a stranger accept an invite or learn what the bot is.
func isStart(update *models.Update) bool {
	name, _, ok := command(update)
	return ok && name == "start"
}
