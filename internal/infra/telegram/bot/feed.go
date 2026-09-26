package bot

import (
	"context"
	"errors"
	"log/slog"
	"strconv"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/sharing"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/i18n"
)

const feedLimit = 10

const (
	actionTake     = "tk"
	actionSendFile = "pl"
)

func (h *Handler) feedView(ctx context.Context) view {
	c := texts(ctx)
	back := backRow(ctx, place{screen: screenHome})
	entries, err := h.ViewFeed.Execute(ctx, feedLimit)
	if err != nil {
		slog.Error("shared_feed", "error", err)
		return view{text: c.FeedFailed(), rows: [][]models.InlineKeyboardButton{back}}
	}
	if len(entries) == 0 {
		return view{text: c.FeedEmpty(), rows: [][]models.InlineKeyboardButton{back}}
	}

	rows := make([][]models.InlineKeyboardButton, 0, len(entries)+1)
	for i, entry := range entries {
		take := models.InlineKeyboardButton{Text: c.TakeButton(i + 1), CallbackData: callbackData(actionTake, entry.Track.ID)}
		if entry.InLibrary {
			take.Text = c.InLibraryButton(i + 1)
		}
		rows = append(rows, []models.InlineKeyboardButton{
			take,
			{Text: c.SendFileButton(i + 1), CallbackData: callbackData(actionSendFile, entry.Track.ID)},
		})
	}
	return view{text: c.Feed(entries), rows: append(rows, back)}
}

func (h *Handler) handleTake(ctx context.Context, b *bot.Bot, query *models.CallbackQuery, sharedTrackID uint) {
	c := texts(ctx)
	err := h.TakeTrack.Execute(ctx, sharedTrackID)
	switch {
	case err == nil:
		answerCallback(ctx, b, query.ID, c.Taken())
	case errors.Is(err, library.ErrAlreadyInLibrary):
		answerCallback(ctx, b, query.ID, c.AlreadyInLibrary())
	case errors.Is(err, sharing.ErrNotShared):
		answerCallback(ctx, b, query.ID, c.NotShared())
	default:
		slog.Error("take", "error", err)
		answerCallback(ctx, b, query.ID, c.TryLater())
		return
	}
	if msg := query.Message.Message; msg != nil {
		h.show(ctx, b, messageRef{chatID: msg.Chat.ID, messageID: msg.ID}, place{screen: screenFeed}, "")
	}
}

// handleSendFile sends to the private chat: the button may be in an inline
// message, where the bot cannot post.
func (h *Handler) handleSendFile(ctx context.Context, b *bot.Bot, query *models.CallbackQuery, sharedTrackID uint) {
	c := texts(ctx)
	err := h.sendFile(ctx, query.From.ID, sharedTrackID)
	switch {
	case err == nil:
		answerCallback(ctx, b, query.ID, "")
	case errors.Is(err, sharing.ErrNotShared):
		answerCallback(ctx, b, query.ID, c.NotShared())
	case errors.Is(err, ErrFileTooLarge):
		answerCallback(ctx, b, query.ID, c.FileTooLarge())
	default:
		slog.Error("send_shared_file", "error", err)
		answerCallback(ctx, b, query.ID, c.TryLater())
	}
}

// sendFile uploads the Track only once: the file serves its copies too.
func (h *Handler) sendFile(ctx context.Context, chatID int64, sharedTrackID uint) error {
	track, path, err := h.GetTrackAudio.Execute(ctx, sharedTrackID)
	if err != nil {
		return err
	}
	file, err := h.Files.For(ctx, track.ID)
	if err != nil {
		return err
	}
	if file != nil {
		return h.Sender.Send(ctx, chatID, file)
	}
	posted, err := h.Sender.Post(ctx, chatID, path, track)
	if err != nil {
		return err
	}
	return h.Files.Remember(ctx, *posted)
}

func (h *Handler) handleInlineFeed(ctx context.Context, b *bot.Bot, queryID string) {
	c := texts(ctx)
	entries, err := h.ViewFeed.Execute(ctx, feedLimit)
	if err != nil {
		slog.Error("shared_feed", "error", err)
		answerInline(ctx, b, queryID, article("error", c.FeedFailedArticle()))
		return
	}
	if len(entries) == 0 {
		answerInline(ctx, b, queryID, article("empty-feed", c.FeedEmptyArticle()))
		return
	}

	results := make([]models.InlineQueryResult, 0, len(entries)+1)
	results = append(results, article("shared-list", c.FeedList(entries)))
	for i := range entries {
		entry := &entries[i]
		id := "shared-" + strconv.FormatUint(uint64(entry.Track.ID), 10)
		caption := c.FeedCaption(entry)
		if file := h.fileOf(ctx, &entry.Track); file != nil {
			results = append(results, cachedFileResult(id, file, caption))
			continue
		}
		results = append(results, &models.InlineQueryResultArticle{
			ID:          id,
			Title:       i18n.ResultTitle(entry.Track.Artist, entry.Track.Title),
			Description: c.SharedBy(&entry.Author),
			InputMessageContent: &models.InputTextMessageContent{
				MessageText: caption,
				ParseMode:   models.ParseModeHTML,
			},
		})
	}
	answerInline(ctx, b, queryID, results...)
}

func (h *Handler) topView(ctx context.Context) view {
	c := texts(ctx)
	back := backRow(ctx, place{screen: screenHome})
	top, err := h.GetTop.Execute(ctx)
	if err != nil {
		slog.Error("get_top", "error", err)
		return view{text: c.TopFailed(), rows: [][]models.InlineKeyboardButton{back}}
	}
	return view{text: c.Top(top), rows: [][]models.InlineKeyboardButton{back}}
}

func (h *Handler) handleInlineTop(ctx context.Context, b *bot.Bot, queryID string) {
	c := texts(ctx)
	top, err := h.GetTop.Execute(ctx)
	if err != nil {
		slog.Error("get_top", "error", err)
		answerInline(ctx, b, queryID, article("error", c.TopFailedArticle()))
		return
	}
	answerInline(ctx, b, queryID, article("top", c.TopArticle(top)))
}
