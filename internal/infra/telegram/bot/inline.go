package bot

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/i18n"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/store"
)

const recentTracksLimit = 10

func (h *Handler) handleInlineQuery(ctx context.Context, b *bot.Bot, update *models.Update) {
	queryID := update.InlineQuery.ID
	switch strings.TrimSpace(strings.ToLower(update.InlineQuery.Query)) {
	case "np":
		h.handleNowPlaying(ctx, b, queryID)
	case "recent", "last":
		h.handleRecentlyPlayed(ctx, b, queryID)
	case "shared":
		h.handleInlineFeed(ctx, b, queryID)
	case "top":
		h.handleInlineTop(ctx, b, queryID)
	default:
		h.answerHints(ctx, b, queryID)
	}
}

// answerHints: a chosen hint sends a note whose button types the command in.
func (h *Handler) answerHints(ctx context.Context, b *bot.Bot, queryID string) {
	c := texts(ctx)
	hints := c.Hints(h.botName(ctx, b))
	results := make([]models.InlineQueryResult, 0, len(hints))
	for _, hint := range hints {
		result := article("hint-"+hint.Command, hint.Article)
		result.ReplyMarkup = &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{{
			{Text: c.Try(), SwitchInlineQueryCurrentChat: &hint.Command},
		}}}
		results = append(results, result)
	}
	answerInline(ctx, b, queryID, results...)
}

func (h *Handler) handleNowPlaying(ctx context.Context, b *bot.Bot, queryID string) {
	c := texts(ctx)
	track, err := h.GetNowPlaying.Execute(ctx)
	switch {
	case errors.Is(err, repositories.ErrNavidromeAccountNotFound):
		answerInline(ctx, b, queryID, article("no-navidrome-account", c.NoNavidromeAccount(h.botName(ctx, b))))
		return
	case err != nil:
		slog.Error("get_now_playing", "error", err)
		answerInline(ctx, b, queryID, article("error", c.NowPlayingFailed()))
		return
	case track == nil:
		answerInline(ctx, b, queryID, article("nothing-playing", c.NothingPlaying()))
		return
	}

	var markup models.ReplyMarkup
	if track.ShareableTrackID != 0 {
		markup = &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{{
			{Text: c.ShareButton(), CallbackData: callbackData(actionShareTrack, track.ShareableTrackID)},
		}}}
	}
	if file := h.fileOf(ctx, track.Track); file != nil {
		answerInline(ctx, b, queryID, withMarkup(cachedFileResult(track.ID, file, c.NowPlaying(track)), markup))
		return
	}
	result := article(track.ID, c.NowPlayingArticle(track))
	result.ReplyMarkup = markup
	answerInline(ctx, b, queryID, result)
}

func (h *Handler) handleRecentlyPlayed(ctx context.Context, b *bot.Bot, queryID string) {
	c := texts(ctx)
	tracks, err := h.GetRecentlyPlayed.Execute(ctx, recentTracksLimit)
	switch {
	case errors.Is(err, repositories.ErrNavidromeAccountNotFound):
		answerInline(ctx, b, queryID, article("no-navidrome-account", c.NoNavidromeAccount(h.botName(ctx, b))))
		return
	case err != nil:
		slog.Error("get_recently_played", "error", err)
		answerInline(ctx, b, queryID, article("error", c.HistoryFailed()))
		return
	case len(tracks) == 0:
		answerInline(ctx, b, queryID, article("nothing-played", c.HistoryEmpty()))
		return
	}

	now := time.Now()
	results := make([]models.InlineQueryResult, 0, len(tracks)+1)
	results = append(results, article("recent-list", c.RecentList(tracks, now)))
	for i := range tracks {
		track := &tracks[i]
		caption := i18n.TrackCaption(track.Artist, track.Title)
		if file := h.fileOf(ctx, track.Track); file != nil {
			results = append(results, cachedFileResult("recent-"+track.ID, file, caption))
			continue
		}
		results = append(results, &models.InlineQueryResultArticle{
			ID:          "recent-" + track.ID,
			Title:       i18n.ResultTitle(track.Artist, track.Title),
			Description: c.RecentDescription(track, now),
			InputMessageContent: &models.InputTextMessageContent{
				MessageText: caption,
				ParseMode:   models.ParseModeHTML,
			},
		})
	}
	answerInline(ctx, b, queryID, results...)
}

func article(id string, a i18n.Article) *models.InlineQueryResultArticle {
	return &models.InlineQueryResultArticle{
		ID:          id,
		Title:       a.Title,
		Description: a.Description,
		InputMessageContent: &models.InputTextMessageContent{
			MessageText: a.Message,
			ParseMode:   models.ParseModeHTML,
		},
	}
}

func cachedFileResult(id string, file *store.File, caption string) models.InlineQueryResult {
	if file.Kind == store.FileDocument {
		return &models.InlineQueryResultCachedDocument{
			ID:             id,
			Title:          "🎧",
			DocumentFileID: file.ID,
			Caption:        caption,
			ParseMode:      models.ParseModeHTML,
		}
	}
	return &models.InlineQueryResultCachedAudio{
		ID:          id,
		AudioFileID: file.ID,
		Caption:     caption,
		ParseMode:   models.ParseModeHTML,
	}
}

func withMarkup(result models.InlineQueryResult, markup models.ReplyMarkup) models.InlineQueryResult {
	switch r := result.(type) {
	case *models.InlineQueryResultCachedAudio:
		r.ReplyMarkup = markup
	case *models.InlineQueryResultCachedDocument:
		r.ReplyMarkup = markup
	}
	return result
}

func answerInline(
	ctx context.Context,
	b *bot.Bot,
	inlineQueryID string,
	results ...models.InlineQueryResult,
) {
	_, err := b.AnswerInlineQuery(ctx, &bot.AnswerInlineQueryParams{
		InlineQueryID: inlineQueryID,
		Results:       results,
		CacheTime:     1,
		IsPersonal:    true,
	})
	if err != nil {
		slog.Error("answer_inline_query", "error", err)
	}
}
