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

// Sharing is /share and the Share buttons.
type Sharing struct {
	Telegram         *Telegram
	ShowShareOptions *share_tracks.ShowShareOptions
	ShareTrack       *share_tracks.ShareTrack
	ShareAlbum       *share_tracks.ShareAlbum
	UnshareTrack     *share_tracks.UnshareTrack
	UnshareAlbum     *share_tracks.UnshareAlbum
}

const (
	actionShareTrack   = "sh:t"
	actionShareAlbum   = "sh:a"
	actionUnshareTrack = "un:t"
	actionUnshareAlbum = "un:a"
)

func (s *Sharing) handleShare(ctx context.Context, _ *bot.Bot, update *models.Update) {
	c := texts(ctx)
	msg := update.Message
	reply := msg.ReplyToMessage
	file, _, ok := audioFile(reply)
	if !ok {
		logUnsent(s.Telegram.sendText(ctx, msg.Chat.ID, c.ShareUsage()))
		return
	}

	state, err := s.ShowShareOptions.Execute(ctx, provider.TrackRef{Provider: tgprovider.Name, ID: file.UniqueID})
	switch {
	case err == nil:
		logUnsent(s.Telegram.sendKeyboard(ctx, msg.Chat.ID, c.ShareWhat(), shareKeyboard(c, state)))
	case errors.Is(err, library.ErrInboxTrack):
		logUnsent(s.Telegram.sendText(ctx, msg.Chat.ID, c.InboxNotShareable()))
	case errors.Is(err, repositories.ErrSourceNotFound):
		logUnsent(s.Telegram.sendText(ctx, msg.Chat.ID, c.ShareUsage()))
	default:
		slog.Error("find_share_state", "error", err)
		logUnsent(s.Telegram.sendText(ctx, msg.Chat.ID, c.TrackNotFound()))
	}
}

func shareKeyboard(c i18n.Catalog, state *share_tracks.ShareState) *models.InlineKeyboardMarkup {
	track := models.InlineKeyboardButton{Text: c.ShareTrack(), CallbackData: callbackData(actionShareTrack, state.TrackID), Style: stylePrimary}
	if state.Shared {
		track = models.InlineKeyboardButton{Text: c.UnshareTrack(), CallbackData: callbackData(actionUnshareTrack, state.TrackID), Style: styleDanger}
	}
	row := []models.InlineKeyboardButton{track}

	if state.HasAlbum {
		album := models.InlineKeyboardButton{Text: c.ShareAlbum(), CallbackData: callbackData(actionShareAlbum, state.TrackID)}
		if state.AlbumShared {
			album = models.InlineKeyboardButton{Text: c.UnshareAlbum(), CallbackData: callbackData(actionUnshareAlbum, state.TrackID), Style: styleDanger}
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

func (s *Sharing) shareTrack(ctx context.Context, query *models.CallbackQuery, trackID uint) {
	result, err := s.ShareTrack.Execute(ctx, trackID)
	s.shared(ctx, query, result, err)
}

func (s *Sharing) shareAlbum(ctx context.Context, query *models.CallbackQuery, trackID uint) {
	result, err := s.ShareAlbum.Execute(ctx, trackID)
	s.shared(ctx, query, result, err)
}

func (s *Sharing) unshareTrack(ctx context.Context, query *models.CallbackQuery, trackID uint) {
	state, err := s.UnshareTrack.Execute(ctx, trackID)
	s.answerShare(ctx, query, texts(ctx).Unshared(), state, err)
}

func (s *Sharing) unshareAlbum(ctx context.Context, query *models.CallbackQuery, trackID uint) {
	state, err := s.UnshareAlbum.Execute(ctx, trackID)
	s.answerShare(ctx, query, texts(ctx).Unshared(), state, err)
}

func (s *Sharing) shared(ctx context.Context, query *models.CallbackQuery, result *share_tracks.ShareResult, err error) {
	if err != nil {
		s.answerShare(ctx, query, "", nil, err)
		return
	}
	s.answerShare(ctx, query, texts(ctx).ShareResult(result), result.State, nil)
}

func (s *Sharing) answerShare(ctx context.Context, query *models.CallbackQuery, text string, state *share_tracks.ShareState, err error) {
	c := texts(ctx)
	if err != nil {
		s.Telegram.answerCallback(ctx, query.ID, s.Telegram.shareFailure(c, query.Data, err))
		return
	}
	s.Telegram.answerCallback(ctx, query.ID, text)
	s.Telegram.editKeyboard(ctx, query, shareKeyboard(c, state))
}

func (t *Telegram) shareFailure(c i18n.Catalog, data string, err error) string {
	var full *library.QuotaExceededError
	switch {
	case errors.As(err, &full):
		return c.SharedLibraryFull(t.AdminContact)
	case errors.Is(err, library.ErrNotKeptTrack):
		return c.NotOwnTrack()
	case errors.Is(err, library.ErrInboxTrack):
		return c.InboxNotShareable()
	default:
		slog.Error("share_callback", "data", data, "error", err)
		return c.TryLater()
	}
}
