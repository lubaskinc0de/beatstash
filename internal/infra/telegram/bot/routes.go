package bot

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/i18n"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/window"
)

const pollTimeout = time.Minute

func Options(ids common.IDProvider, users *Users, windows *window.Windows, bundle *i18n.Bundle) []bot.Option {
	return []bot.Option{
		bot.WithHTTPClient(pollTimeout, window.Watch(windows, &http.Client{Timeout: pollTimeout})),
		// The app calls getMe itself: it needs the username.
		bot.WithSkipGetMe(),
		bot.WithAllowedUpdates(bot.AllowedUpdates{
			"message",
			"inline_query",
			"callback_query",
			"chosen_inline_result",
		}),
		bot.WithMiddlewares(window.CountArrivals(windows), senderMiddleware, languageMiddleware(users, bundle), membersOnly(ids)),
	}
}

func isInlineQuery(update *models.Update) bool {
	return update.InlineQuery != nil
}

func isCallbackQuery(update *models.Update) bool {
	return update.CallbackQuery != nil
}

func (h *Handler) handleCallbackQuery(ctx context.Context, _ *bot.Bot, update *models.Update) {
	query := update.CallbackQuery
	action, arg, _ := strings.Cut(query.Data, ":")
	if msg := query.Message.Message; msg != nil {
		if handle := h.windowAction(action); handle != nil {
			handle(ctx, windowCallback{messageRef: messageRef{chatID: msg.Chat.ID, messageID: msg.ID}, query: query}, arg)
			return
		}
	}
	action, trackID, ok := parseCallback(query.Data)
	if handle := h.trackAction(action); ok && handle != nil {
		handle(ctx, query, trackID)
		return
	}
	h.Telegram.answerCallback(ctx, query.ID, "")
}

// windowAction returns nil for a button that is not the window's.
func (h *Handler) windowAction(action string) func(context.Context, windowCallback, string) {
	switch action {
	case actionGo:
		return h.goTo
	case actionLanguage:
		return h.Home.chooseLanguage
	case actionStartImport:
		return h.Imports.startImport
	case actionDisconnect:
		return h.Imports.disconnect
	case actionQuota:
		return h.Quotas.chooseQuota
	default:
		return nil
	}
}

// trackAction returns nil for a button that is not about a Track.
func (h *Handler) trackAction(action string) func(context.Context, *models.CallbackQuery, uint) {
	switch action {
	case actionShareTrack:
		return h.Sharing.shareTrack
	case actionShareAlbum:
		return h.Sharing.shareAlbum
	case actionUnshareTrack:
		return h.Sharing.unshareTrack
	case actionUnshareAlbum:
		return h.Sharing.unshareAlbum
	case actionTake:
		return h.Feed.handleTake
	case actionSendFile:
		return h.Feed.handleSendFile
	default:
		return nil
	}
}
