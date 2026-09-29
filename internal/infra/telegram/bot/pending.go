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

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/i18n"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/store"
)

// A pending result stands for a Track without a Telegram file: once chosen,
// the bot uploads the file and turns the sent message into audio. Its id
// carries the Track, since chosen_inline_result reports nothing else.
const (
	pendingPrefix = "pending:"
	pendingShare  = ":share"
)

// actionWait is the button of a pending message without Share: without a
// keyboard Telegram gives no inline_message_id to edit.
const actionWait = "wait"

// pendingResultID: n tells the results of one answer apart; shareable
// keeps the Share button on the audio.
func pendingResultID(trackID uint, n int, shareable bool) string {
	id := fmt.Sprintf("%s%d:%d", pendingPrefix, trackID, n)
	if shareable {
		id += pendingShare
	}
	return id
}

func parsePendingResult(id string) (trackID uint, shareable bool, ok bool) {
	rest, ok := strings.CutPrefix(id, pendingPrefix)
	if !ok {
		return 0, false, false
	}
	rest, shareable = strings.CutSuffix(rest, pendingShare)
	track, _, _ := strings.Cut(rest, ":")
	n, err := strconv.ParseUint(track, 10, 64)
	if err != nil {
		return 0, false, false
	}
	return uint(n), shareable, true
}

// pending tells whether choosing the Track can bring its file: it is on
// the bot's disk and there is a storage chat to upload it to.
func (h *Handler) pending(track *library.Track) bool {
	return track != nil && !track.Attached() && h.StorageChatID != 0
}

func pendingResult(id string, a i18n.Article, track *library.Track, markup models.ReplyMarkup) *models.InlineQueryResultArticle {
	a.Message = i18n.PendingCaption(track.Artist, track.Title)
	result := article(id, a)
	result.ReplyMarkup = markup
	if markup == nil {
		result.ReplyMarkup = &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{{
			{Text: "⏳", CallbackData: actionWait},
		}}}
	}
	return result
}

func isChosenInlineResult(update *models.Update) bool {
	return update.ChosenInlineResult != nil
}

func (h *Handler) handleChosenInlineResult(ctx context.Context, b *bot.Bot, update *models.Update) {
	chosen := update.ChosenInlineResult
	trackID, shareable, ok := parsePendingResult(chosen.ResultID)
	if !ok || chosen.InlineMessageID == "" {
		return
	}
	c := texts(ctx)
	var markup models.ReplyMarkup
	if shareable {
		markup = shareTrackKeyboard(c, trackID)
	}

	track, path, err := h.GetTrackFile.Execute(ctx, trackID)
	if err != nil {
		slog.Error("get_track_file", "track_id", trackID, "error", err)
		editInlineText(ctx, b, chosen.InlineMessageID, c.NotSentNote(false), markup)
		return
	}
	file, _, err := h.fileFor(ctx, h.StorageChatID, track, path)
	if err != nil {
		slog.Error("store_chosen_track", "track_id", trackID, "error", err)
		editInlineText(ctx, b, chosen.InlineMessageID,
			c.NotSentCaption(track.Artist, track.Title, errors.Is(err, ErrFileTooLarge)), markup)
		return
	}
	_, err = b.EditMessageMedia(ctx, &bot.EditMessageMediaParams{
		InlineMessageID: chosen.InlineMessageID,
		Media:           inputMedia(file, i18n.TrackCaption(track.Artist, track.Title)),
		ReplyMarkup:     markup,
	})
	if err != nil {
		slog.Error("edit_message_media", "error", err)
	}
}

func inputMedia(file *store.File, caption string) models.InputMedia {
	if file.Kind == store.FileDocument {
		return &models.InputMediaDocument{Media: file.ID, Caption: caption, ParseMode: models.ParseModeHTML}
	}
	return &models.InputMediaAudio{Media: file.ID, Caption: caption, ParseMode: models.ParseModeHTML}
}

func editInlineText(ctx context.Context, b *bot.Bot, inlineMessageID, text string, markup models.ReplyMarkup) {
	_, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
		InlineMessageID: inlineMessageID,
		Text:            text,
		ParseMode:       models.ParseModeHTML,
		ReplyMarkup:     markup,
	})
	if err != nil {
		slog.Error("edit_inline_message", "error", err)
	}
}
