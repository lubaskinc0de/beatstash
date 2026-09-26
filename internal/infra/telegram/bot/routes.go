package bot

import (
	"context"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/i18n"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/store"
)

func Options(ids common.IDProvider, users *store.Users, bundle *i18n.Bundle) []bot.Option {
	return []bot.Option{
		bot.WithAllowedUpdates(bot.AllowedUpdates{
			"message",
			"inline_query",
			"callback_query",
		}),
		bot.WithMiddlewares(senderMiddleware, languageMiddleware(users, bundle), membersOnly(ids)),
	}
}

// Register adds the routes in priority order: the bot runs the first match.
// Everything but /start and /share happens in the window.
func (h *Handler) Register(b *bot.Bot) {
	b.RegisterHandlerMatchFunc(hasAudio, h.handleAudio)
	b.RegisterHandlerMatchFunc(isCommand("start"), h.handleStart)
	b.RegisterHandlerMatchFunc(isCommand("share"), h.handleShare)
	b.RegisterHandlerMatchFunc(isText, h.handleText)
	b.RegisterHandlerMatchFunc(isInlineQuery, h.handleInlineQuery)
	b.RegisterHandlerMatchFunc(isCallbackQuery, h.handleCallbackQuery)
}

func isInlineQuery(update *models.Update) bool {
	return update.InlineQuery != nil
}

func isCallbackQuery(update *models.Update) bool {
	return update.CallbackQuery != nil
}

type callbackHandler func(ctx context.Context, b *bot.Bot, query *models.CallbackQuery, id uint)

func (h *Handler) callbackHandler(action string) (callbackHandler, bool) {
	switch action {
	case actionShareTrack, actionShareAlbum, actionUnshareTrack, actionUnshareAlbum:
		return h.shareCallback(action), true
	case actionTake:
		return h.handleTake, true
	case actionSendFile:
		return h.handleSendFile, true
	}
	return nil, false
}

func (h *Handler) handleCallbackQuery(ctx context.Context, b *bot.Bot, update *models.Update) {
	query := update.CallbackQuery
	action, arg, _ := strings.Cut(query.Data, ":")
	if handle, ok := h.windowAction(action); ok {
		h.handleWindowCallback(ctx, b, query, handle, arg)
		return
	}
	action, id, ok := parseCallback(query.Data)
	if handle, known := h.callbackHandler(action); ok && known {
		handle(ctx, b, query, id)
		return
	}
	answerCallback(ctx, b, query.ID, "")
}
