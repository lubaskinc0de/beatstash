package telegram

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/application"
	"github.com/lubaskinc0de/navidrome-tg/internal/database"
	"github.com/lubaskinc0de/navidrome-tg/internal/entities"
)

func withUser(ctx context.Context, user *entities.User) context.Context {
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

func UserMiddleware(users *database.UserRepository, allowedUserIds []uint64) bot.Middleware {
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

			if !slices.Contains(allowedUserIds, telegramID) {
				slog.Info("access_denied", "uid", telegramID)
				return
			}

			user, err := users.GetById(ctx, telegramID)
			if err != nil && !errors.Is(err, application.ErrUserNotFound) {
				slog.Error("get_user", "error", err)
				return
			}

			if errors.Is(err, application.ErrUserNotFound) {
				user = &entities.User{
					TelegramID: telegramID,
					Username:   from.Username,
					CreatedAt:  time.Now(),
				}

				if err := users.Save(ctx, user); err != nil {
					slog.Error("create_user", "error", err)
					return
				}
			}

			ctx = withUser(ctx, user)

			next(ctx, b, update)
		}
	}
}
