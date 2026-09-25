package bot

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/application"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

const feedLimit = 10

const (
	actionTake     = "tk"
	actionSendFile = "pl"
)

const emptyFeed = "💤 Пока никто ничего не расшарил. Ответьте /share на аудиосообщение со своим треком, чтобы стать первым"

func (h *Handler) handleSharedFeed(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID

	entries, err := h.Sharing.Feed(ctx, feedLimit)
	if err != nil {
		slog.Error("shared_feed", "error", err)
		sendText(ctx, b, chatID, "⚠️ Не удалось получить ленту, попробуйте позже")
		return
	}
	if len(entries) == 0 {
		sendText(ctx, b, chatID, emptyFeed)
		return
	}

	rows := make([][]models.InlineKeyboardButton, 0, len(entries))
	for i, entry := range entries {
		n := strconv.Itoa(i + 1)
		rows = append(rows, []models.InlineKeyboardButton{
			{Text: n + ". ➕ Взять себе", CallbackData: callbackData(actionTake, entry.Track.ID)},
			{Text: n + ". ▶️ Прислать файл", CallbackData: callbackData(actionSendFile, entry.Track.ID)},
		})
	}
	sendKeyboard(ctx, b, chatID, feedText(entries), &models.InlineKeyboardMarkup{InlineKeyboard: rows})
}

func feedText(entries []library.FeedEntry) string {
	var b strings.Builder
	b.WriteString("🔗 <b>Свежее в общей библиотеке:</b>\n\n")
	for i, entry := range entries {
		fmt.Fprintf(&b, "%d. %s\n     <i>расшарил %s</i>\n", i+1, trackLine(&entry.Track), html.EscapeString(authorName(&entry.Author)))
	}
	return b.String()
}

func trackLine(track *domain.Track) string {
	return fmt.Sprintf("<b>%s</b> — %s", html.EscapeString(track.Artist), html.EscapeString(track.Title))
}

func (h *Handler) handleInlineFeed(ctx context.Context, b *bot.Bot, update *models.Update) {
	queryID := update.InlineQuery.ID

	entries, err := h.Sharing.Feed(ctx, feedLimit)
	if err != nil {
		slog.Error("shared_feed", "error", err)
		answerInlineArticle(ctx, b, queryID, "error", "⚠️ Лента недоступна", "Не удалось получить расшаренное", "⚠️ Не удалось получить расшаренное")
		return
	}
	if len(entries) == 0 {
		answerInlineArticle(ctx, b, queryID, "empty-feed", "💤 Пока ничего не расшарено", "Расшарьте трек командой /share", emptyFeed)
		return
	}

	results := make([]models.InlineQueryResult, 0, len(entries)+1)
	results = append(results, &models.InlineQueryResultArticle{
		ID:          "shared-list",
		Title:       fmt.Sprintf("🔗 Свежее в общей: %s", tracksCount(len(entries))),
		Description: "Отправить весь список одним сообщением",
		InputMessageContent: &models.InputTextMessageContent{
			MessageText: feedText(entries),
			ParseMode:   models.ParseModeHTML,
		},
	})
	for _, entry := range entries {
		id := "shared-" + strconv.FormatUint(uint64(entry.Track.ID), 10)
		caption := fmt.Sprintf("🎧 %s\n🔗 расшарил %s", trackLine(&entry.Track), html.EscapeString(authorName(&entry.Author)))
		if entry.TelegramFile != nil {
			results = append(results, cachedFileResult(id, entry.TelegramFile, caption))
			continue
		}
		results = append(results, &models.InlineQueryResultArticle{
			ID:          id,
			Title:       fmt.Sprintf("🎧 %s — %s", entry.Track.Artist, entry.Track.Title),
			Description: "расшарил " + authorName(&entry.Author),
			InputMessageContent: &models.InputTextMessageContent{
				MessageText: caption,
				ParseMode:   models.ParseModeHTML,
			},
		})
	}
	answerInline(ctx, b, queryID, results...)
}

func (h *Handler) handleTake(ctx context.Context, b *bot.Bot, query *models.CallbackQuery, sharedTrackID uint) {
	err := h.Sharing.Take(ctx, sharedTrackID)
	switch {
	case err == nil:
		answerCallback(ctx, b, query.ID, "➕ Трек в вашей библиотеке")
	case errors.Is(err, library.ErrAlreadyInLibrary):
		answerCallback(ctx, b, query.ID, "Этот трек уже есть у вас")
	case errors.Is(err, library.ErrNotShared):
		answerCallback(ctx, b, query.ID, "Трек больше не в общей библиотеке")
	default:
		slog.Error("take", "error", err)
		answerCallback(ctx, b, query.ID, "⚠️ Не получилось, попробуйте позже")
	}
}

// handleSendFile sends to the private chat: the button may sit in a
// message sent through inline mode, where the bot cannot post.
func (h *Handler) handleSendFile(ctx context.Context, b *bot.Bot, query *models.CallbackQuery, sharedTrackID uint) {
	file, err := h.Sharing.SharedFile(ctx, sharedTrackID)
	switch {
	case errors.Is(err, library.ErrNotShared):
		answerCallback(ctx, b, query.ID, "Трек больше не в общей библиотеке")
		return
	case errors.Is(err, application.ErrNoTelegramFile):
		answerCallback(ctx, b, query.ID, "Файла этого трека в Telegram нет")
		return
	case err != nil:
		slog.Error("shared_file", "error", err)
		answerCallback(ctx, b, query.ID, "⚠️ Не получилось, попробуйте позже")
		return
	}
	answerCallback(ctx, b, query.ID, "")
	sendFile(ctx, b, query.From.ID, file)
}

func sendFile(ctx context.Context, b *bot.Bot, chatID int64, file *domain.TelegramFile) {
	var err error
	if file.Kind == domain.TelegramFileDocument {
		_, err = b.SendDocument(ctx, &bot.SendDocumentParams{ChatID: chatID, Document: &models.InputFileString{Data: file.ID}})
	} else {
		_, err = b.SendAudio(ctx, &bot.SendAudioParams{ChatID: chatID, Audio: &models.InputFileString{Data: file.ID}})
	}
	if err != nil {
		slog.Error("send_file", "error", err)
	}
}
