package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/share_tracks"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
	tgprovider "github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/provider"
)

const shareUsage = "Ответьте /share на аудиосообщение с треком из вашей библиотеки: я предложу расшарить его или весь альбом"

const (
	actionShareTrack   = "sh:t"
	actionShareAlbum   = "sh:a"
	actionUnshareTrack = "un:t"
	actionUnshareAlbum = "un:a"
)

func (h *Handler) handleShare(ctx context.Context, b *bot.Bot, update *models.Update) {
	msg := update.Message
	reply := msg.ReplyToMessage
	file, _, ok := audioFile(reply)
	if !ok {
		sendText(ctx, b, msg.Chat.ID, shareUsage)
		return
	}

	state, err := h.ShowShareOptions.Execute(ctx, provider.TrackRef{Provider: tgprovider.Name, ID: file.UniqueID})
	switch {
	case err == nil:
		sendKeyboard(ctx, b, msg.Chat.ID, "Что расшарить?", shareKeyboard(state))
	case errors.Is(err, library.ErrInboxTrack):
		sendText(ctx, b, msg.Chat.ID, "🗂 Трек из Inbox нельзя расшарить: у него нет исполнителя или названия")
	case errors.Is(err, repositories.ErrSourceNotFound):
		sendText(ctx, b, msg.Chat.ID, shareUsage)
	default:
		slog.Error("find_share_state", "error", err)
		sendText(ctx, b, msg.Chat.ID, "⚠️ Не удалось найти трек, попробуйте позже")
	}
}

func shareKeyboard(state *share_tracks.ShareState) *models.InlineKeyboardMarkup {
	track := models.InlineKeyboardButton{Text: "🔗 Трек", CallbackData: callbackData(actionShareTrack, state.TrackID)}
	if state.Shared {
		track = models.InlineKeyboardButton{Text: "🔒 Снять Share", CallbackData: callbackData(actionUnshareTrack, state.TrackID)}
	}
	row := []models.InlineKeyboardButton{track}

	if state.HasAlbum {
		album := models.InlineKeyboardButton{Text: "💿 Альбом целиком", CallbackData: callbackData(actionShareAlbum, state.TrackID)}
		if state.AlbumShared {
			album = models.InlineKeyboardButton{Text: "🔒 Снять альбом", CallbackData: callbackData(actionUnshareAlbum, state.TrackID)}
		}
		row = append(row, album)
	}
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{row}}
}

func callbackData(action string, id uint) string {
	return action + ":" + strconv.FormatUint(uint64(id), 10)
}

func parseCallback(data string) (action string, id uint, ok bool) {
	i := strings.LastIndexByte(data, ':')
	if i < 0 {
		return "", 0, false
	}
	n, err := strconv.ParseUint(data[i+1:], 10, 64)
	if err != nil {
		return "", 0, false
	}
	return data[:i], uint(n), true
}

func (h *Handler) shareCallback(action string) callbackHandler {
	return func(ctx context.Context, b *bot.Bot, query *models.CallbackQuery, trackID uint) {
		h.handleShareCallback(ctx, b, query, action, trackID)
	}
}

func (h *Handler) handleShareCallback(ctx context.Context, b *bot.Bot, query *models.CallbackQuery, action string, trackID uint) {
	var (
		text  string
		state *share_tracks.ShareState
		err   error
	)
	switch action {
	case actionShareTrack:
		text, state, err = shareOutcome(h.ShareTrack.Execute(ctx, trackID))
	case actionShareAlbum:
		text, state, err = shareOutcome(h.ShareAlbum.Execute(ctx, trackID))
	case actionUnshareTrack:
		state, err = h.UnshareTrack.Execute(ctx, trackID)
		text = "🔒 Share снят"
	case actionUnshareAlbum:
		state, err = h.UnshareAlbum.Execute(ctx, trackID)
		text = "🔒 Share снят"
	}

	switch {
	case errors.Is(err, library.ErrNotOwnTrack):
		answerCallback(ctx, b, query.ID, "Этот трек не из вашей библиотеки")
		return
	case errors.Is(err, library.ErrInboxTrack):
		answerCallback(ctx, b, query.ID, "Трек из Inbox нельзя расшарить")
		return
	case err != nil:
		slog.Error("share_callback", "action", action, "error", err)
		answerCallback(ctx, b, query.ID, "⚠️ Не получилось, попробуйте позже")
		return
	}
	answerCallback(ctx, b, query.ID, text)

	// A message sent through inline mode is seen by the whole chat: its
	// button goes away instead of offering to unshare.
	if query.InlineMessageID != "" {
		editKeyboard(ctx, b, query, &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{}})
		return
	}
	editKeyboard(ctx, b, query, shareKeyboard(state))
}

func shareOutcome(result *share_tracks.ShareResult, err error) (string, *share_tracks.ShareState, error) {
	if err != nil {
		return "", nil, err
	}
	return shareResultText(result), result.State, nil
}

func shareResultText(result *share_tracks.ShareResult) string {
	switch {
	case result.Created == 0 && result.AlreadyShared == 0:
		return "Уже расшарено вами"
	case result.AlreadyShared == 0:
		return fmt.Sprintf("🔗 В общей библиотеке: %s", tracksCount(result.Created))
	case result.Created == 0:
		return "Трек уже в общей, расшарил " + authorName(result.Author)
	default:
		return fmt.Sprintf(
			"🔗 В общей библиотеке: %s, ещё %s уже были там",
			tracksCount(result.Created), tracksCount(result.AlreadyShared),
		)
	}
}

func authorName(user *access.User) string {
	switch {
	case user == nil:
		return "кто-то раньше"
	case user.Username != "":
		return "@" + user.Username
	default:
		return "пользователь без username"
	}
}

func tracksCount(n int) string {
	return plural(n, "трек", "трека", "треков")
}

func answerCallback(ctx context.Context, b *bot.Bot, id, text string) {
	_, err := b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{CallbackQueryID: id, Text: text})
	if err != nil {
		slog.Error("answer_callback_query", "error", err)
	}
}

func editKeyboard(ctx context.Context, b *bot.Bot, query *models.CallbackQuery, markup *models.InlineKeyboardMarkup) {
	params := &bot.EditMessageReplyMarkupParams{ReplyMarkup: markup, InlineMessageID: query.InlineMessageID}
	if msg := query.Message.Message; msg != nil {
		params.ChatID = msg.Chat.ID
		params.MessageID = msg.ID
	}
	if _, err := b.EditMessageReplyMarkup(ctx, params); err != nil {
		slog.Error("edit_reply_markup", "error", err)
	}
}

func sendKeyboard(ctx context.Context, b *bot.Bot, chatID int64, text string, markup models.ReplyMarkup) {
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: markup,
	})
	if err != nil {
		slog.Error("send_message", "error", err)
	}
}
