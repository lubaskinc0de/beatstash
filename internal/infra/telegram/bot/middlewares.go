package bot

import (
	"context"
	"errors"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
)

func updateSender(update *models.Update) *models.User {
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

func senderMiddleware(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		from := updateSender(update)
		if from == nil {
			slog.Info("access_denied", "reason", "unknown_update_type")
			return
		}
		next(withSender(ctx, from), b, update)
	}
}

// membersOnly keeps the bot silent to strangers: handlers may answer before
// any interactor turns them away.
func membersOnly(ids common.IDProvider) bot.Middleware {
	return func(next bot.HandlerFunc) bot.HandlerFunc {
		return func(ctx context.Context, b *bot.Bot, update *models.Update) {
			_, err := ids.CurrentUser(ctx)
			switch {
			case err == nil, errors.Is(err, common.ErrNotAuthenticated) && isStart(update):
				next(ctx, b, update)
			case errors.Is(err, common.ErrNotAuthenticated):
				slog.Info("access_denied", "uid", updateSender(update).ID)
			default:
				slog.Error("authenticate", "error", err)
			}
		}
	}
}

// isStart lets a stranger accept an invite or learn what the bot is.
func isStart(update *models.Update) bool {
	name, _, ok := command(update)
	return ok && name == "start"
}
