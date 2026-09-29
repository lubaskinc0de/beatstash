package bot

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/i18n"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/store"
)

func updateSender(update *models.Update) *models.User {
	switch {
	case update.Message != nil:
		return update.Message.From
	case update.InlineQuery != nil:
		return update.InlineQuery.From
	case update.CallbackQuery != nil:
		return &update.CallbackQuery.From
	case update.ChosenInlineResult != nil:
		return &update.ChosenInlineResult.From
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

// languageMiddleware takes the language from the client on first contact;
// afterwards only the switch changes it.
func languageMiddleware(users *store.Users, bundle *i18n.Bundle) bot.Middleware {
	return func(next bot.HandlerFunc) bot.HandlerFunc {
		return func(ctx context.Context, b *bot.Bot, update *models.Update) {
			from := updateSender(update)
			lang := bundle.Match(from.LanguageCode)
			saved, known, err := users.Language(ctx, from.ID)
			switch {
			case err != nil:
				slog.Error("read_language", "error", err)
			case known:
				lang = i18n.Language(saved)
			default:
				if err := users.Meet(ctx, from.ID, string(lang)); err != nil {
					slog.Error("save_language", "error", err)
				}
			}
			next(withTexts(ctx, bundle.For(lang)), b, update)
		}
	}
}

// membersOnly keeps the bot silent to strangers: handlers may answer before
// any interactor turns them away.
func membersOnly(ids common.IDProvider) bot.Middleware {
	return func(next bot.HandlerFunc) bot.HandlerFunc {
		return func(ctx context.Context, b *bot.Bot, update *models.Update) {
			_, err := ids.CurrentUser(ctx)
			switch {
			case err == nil, errors.Is(err, common.ErrNotAuthenticated) && openToStrangers(update):
				next(ctx, b, update)
			case errors.Is(err, common.ErrNotAuthenticated):
				slog.Info("access_denied", "uid", updateSender(update).ID)
			default:
				slog.Error("authenticate", "error", err)
			}
		}
	}
}

// openToStrangers: /start and the language switch.
func openToStrangers(update *models.Update) bool {
	name, _, ok := command(update)
	if ok && name == "start" {
		return true
	}
	if update.CallbackQuery == nil {
		return false
	}
	action, _, _ := strings.Cut(update.CallbackQuery.Data, ":")
	return action == actionLanguage
}

type textsContextKey struct{}

func withTexts(ctx context.Context, c i18n.Catalog) context.Context {
	return context.WithValue(ctx, textsContextKey{}, c)
}

// texts panics without a catalog: the middleware and the Poller always
// put one into ctx.
func texts(ctx context.Context) i18n.Catalog {
	c, ok := ctx.Value(textsContextKey{}).(i18n.Catalog)
	if !ok {
		panic("no texts in the context")
	}
	return c
}
