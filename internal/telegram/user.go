package telegram

import (
	"context"
	"errors"
	"log/slog"
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

func UserMiddleware(users *database.UserRepository) bot.Middleware {
	return func(next bot.HandlerFunc) bot.HandlerFunc {
		return func(
			ctx context.Context,
			b *bot.Bot,
			update *models.Update,
		) {
			if update.Message == nil || update.Message.From == nil {
				next(ctx, b, update)
				return
			}

			telegramID := uint64(update.Message.From.ID)

			user, err := users.GetById(ctx, telegramID)
			if err != nil && !errors.Is(err, application.ErrUserNotFound) {
				slog.Error("get_user", "error", err)
				return
			}

			if errors.Is(err, application.ErrUserNotFound) {
				user = &entities.User{
					TelegramID: update.Message.From.ID,
					Username:   update.Message.From.Username,
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
