package bot

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/share_tracks"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/i18n"
	tgprovider "github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/provider"
)

const (
	actionShareTrack   = "sh:t"
	actionShareAlbum   = "sh:a"
	actionUnshareTrack = "un:t"
	actionUnshareAlbum = "un:a"
)

func (h *Handler) handleShare(ctx context.Context, b *bot.Bot, update *models.Update) {
	c := texts(ctx)
	msg := update.Message
	reply := msg.ReplyToMessage
	file, _, ok := audioFile(reply)
	if !ok {
		sendText(ctx, b, msg.Chat.ID, c.ShareUsage())
		return
	}

	state, err := h.ShowShareOptions.Execute(ctx, provider.TrackRef{Provider: tgprovider.Name, ID: file.UniqueID})
	switch {
	case err == nil:
		sendKeyboard(ctx, b, msg.Chat.ID, c.ShareWhat(), shareKeyboard(c, state))
	case errors.Is(err, library.ErrInboxTrack):
		sendText(ctx, b, msg.Chat.ID, c.InboxNotShareable())
	case errors.Is(err, repositories.ErrSourceNotFound):
		sendText(ctx, b, msg.Chat.ID, c.ShareUsage())
	default:
		slog.Error("find_share_state", "error", err)
		sendText(ctx, b, msg.Chat.ID, c.TrackNotFound())
	}
}

func shareKeyboard(c i18n.Catalog, state *share_tracks.ShareState) *models.InlineKeyboardMarkup {
	track := models.InlineKeyboardButton{Text: c.ShareTrack(), CallbackData: callbackData(actionShareTrack, state.TrackID)}
	if state.Shared {
		track = models.InlineKeyboardButton{Text: c.UnshareTrack(), CallbackData: callbackData(actionUnshareTrack, state.TrackID)}
	}
	row := []models.InlineKeyboardButton{track}

	if state.HasAlbum {
		album := models.InlineKeyboardButton{Text: c.ShareAlbum(), CallbackData: callbackData(actionShareAlbum, state.TrackID)}
		if state.AlbumShared {
			album = models.InlineKeyboardButton{Text: c.UnshareAlbum(), CallbackData: callbackData(actionUnshareAlbum, state.TrackID)}
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
	c := texts(ctx)
	var (
		text   string
		state  *share_tracks.ShareState
		result *share_tracks.ShareResult
		err    error
	)
	switch action {
	case actionShareTrack:
		result, err = h.ShareTrack.Execute(ctx, trackID)
	case actionShareAlbum:
		result, err = h.ShareAlbum.Execute(ctx, trackID)
	case actionUnshareTrack:
		state, err = h.UnshareTrack.Execute(ctx, trackID)
		text = c.Unshared()
	case actionUnshareAlbum:
		state, err = h.UnshareAlbum.Execute(ctx, trackID)
		text = c.Unshared()
	}

	switch {
	case errors.Is(err, library.ErrNotOwnTrack):
		answerCallback(ctx, b, query.ID, c.NotOwnTrack())
		return
	case errors.Is(err, library.ErrInboxTrack):
		answerCallback(ctx, b, query.ID, c.InboxNotShareable())
		return
	case err != nil:
		slog.Error("share_callback", "action", action, "error", err)
		answerCallback(ctx, b, query.ID, c.TryLater())
		return
	}
	if result != nil {
		text, state = c.ShareResult(result), result.State
	}
	answerCallback(ctx, b, query.ID, text)

	// A message sent through inline mode is seen by the whole chat: its
	// button goes away instead of offering to unshare.
	if query.InlineMessageID != "" {
		editKeyboard(ctx, b, query, noKeyboard())
		return
	}
	editKeyboard(ctx, b, query, shareKeyboard(c, state))
}
