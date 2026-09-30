package bot

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/browse_shared"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/show_playing"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/view_top"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/i18n"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/trackfile"
)

// Inline answers inline queries and brings the files of chosen results.
type Inline struct {
	Telegram          *Telegram
	GetNowPlaying     *show_playing.GetNowPlaying
	GetRecentlyPlayed *show_playing.GetRecentlyPlayed
	GetTrackFile      *show_playing.GetTrackFile
	ViewFeed          *browse_shared.ViewFeed
	GetTop            *view_top.GetTop
	Files             *trackfile.Files
	// StorageChatID gets the Tracks without a Telegram file, so inline mode
	// sends them as audio; zero turns it off.
	StorageChatID int64
}

const recentTracksLimit = 10

func (in *Inline) handleInlineQuery(ctx context.Context, _ *bot.Bot, update *models.Update) {
	queryID := update.InlineQuery.ID
	switch strings.TrimSpace(strings.ToLower(update.InlineQuery.Query)) {
	case "np":
		in.handleNowPlaying(ctx, queryID)
	case "recent", "last":
		in.handleRecentlyPlayed(ctx, queryID)
	case "shared":
		in.handleInlineFeed(ctx, queryID)
	case "top":
		in.handleInlineTop(ctx, queryID)
	default:
		in.answerHints(ctx, queryID)
	}
}

// answerHints: a chosen hint sends a note whose button types the command in.
func (in *Inline) answerHints(ctx context.Context, queryID string) {
	c := texts(ctx)
	hints := c.Hints(in.Telegram.BotName)
	results := make([]models.InlineQueryResult, 0, len(hints))
	for _, hint := range hints {
		result := article("hint-"+hint.Command, hint.Article)
		result.ReplyMarkup = &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{{
			{Text: c.Try(), SwitchInlineQueryCurrentChat: &hint.Command},
		}}}
		results = append(results, result)
	}
	in.answerInline(ctx, queryID, results...)
}

func (in *Inline) handleNowPlaying(ctx context.Context, queryID string) {
	c := texts(ctx)
	track, err := in.GetNowPlaying.Execute(ctx)
	switch {
	case errors.Is(err, repositories.ErrNavidromeAccountNotFound):
		in.answerInline(ctx, queryID, article("no-navidrome-account", c.NoNavidromeAccount(in.Telegram.BotName)))
		return
	case err != nil:
		slog.Error("get_now_playing", "error", err)
		in.answerInline(ctx, queryID, article("error", c.NowPlayingFailed()))
		return
	case track == nil:
		in.answerInline(ctx, queryID, article("nothing-playing", c.NothingPlaying()))
		return
	}

	var markup models.ReplyMarkup
	if track.ShareableTrackID != 0 {
		markup = shareTrackKeyboard(c, track.ShareableTrackID)
	}
	if file := in.filesOf(ctx, track.Track).of(track.Track); file != nil {
		in.answerInline(ctx, queryID, withMarkup(cachedFileResult(track.ID, file, c.NowPlaying(track)), markup))
		return
	}
	if in.pending(track.Track) {
		in.answerInline(ctx, queryID, pendingResult(pendingResultID(track.Track.ID, 0, markup != nil), c.NowPlayingArticle(track), track.Track, markup))
		return
	}
	result := article(track.ID, c.NowPlayingArticle(track))
	result.ReplyMarkup = markup
	in.answerInline(ctx, queryID, result)
}

func shareTrackKeyboard(c i18n.Catalog, trackID uint) *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{{
		{Text: c.ShareButton(), CallbackData: callbackData(actionShareTrack, trackID)},
	}}}
}

func (in *Inline) handleRecentlyPlayed(ctx context.Context, queryID string) {
	c := texts(ctx)
	tracks, err := in.GetRecentlyPlayed.Execute(ctx, recentTracksLimit)
	switch {
	case errors.Is(err, repositories.ErrNavidromeAccountNotFound):
		in.answerInline(ctx, queryID, article("no-navidrome-account", c.NoNavidromeAccount(in.Telegram.BotName)))
		return
	case err != nil:
		slog.Error("get_recently_played", "error", err)
		in.answerInline(ctx, queryID, article("error", c.HistoryFailed()))
		return
	case len(tracks) == 0:
		in.answerInline(ctx, queryID, article("nothing-played", c.HistoryEmpty()))
		return
	}

	now := time.Now()
	results := make([]models.InlineQueryResult, 0, len(tracks)+1)
	results = append(results, article("recent-list", c.RecentList(tracks, now)))
	found := make([]*library.Track, 0, len(tracks))
	for i := range tracks {
		found = append(found, tracks[i].Track)
	}
	files := in.filesOf(ctx, found...)
	for i := range tracks {
		track := &tracks[i]
		caption := i18n.TrackCaption(track.Artist, track.Title)
		if file := files.of(track.Track); file != nil {
			results = append(results, cachedFileResult("recent-"+track.ID, file, caption))
			continue
		}
		a := i18n.Article{
			Title:       i18n.ResultTitle(track.Artist, track.Title),
			Description: c.RecentDescription(track, now),
			Message:     caption,
		}
		if in.pending(track.Track) {
			results = append(results, pendingResult(pendingResultID(track.Track.ID, i, false), a, track.Track, nil))
			continue
		}
		results = append(results, article("recent-"+track.ID, a))
	}
	in.answerInline(ctx, queryID, results...)
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

func cachedFileResult(id string, file *trackfile.File, caption string) models.InlineQueryResult {
	if file.Kind == trackfile.FileDocument {
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

func (in *Inline) answerInline(
	ctx context.Context,
	inlineQueryID string,
	results ...models.InlineQueryResult,
) {
	_, err := in.Telegram.Bot.AnswerInlineQuery(ctx, &bot.AnswerInlineQueryParams{
		InlineQueryID: inlineQueryID,
		Results:       results,
		CacheTime:     1,
		IsPersonal:    true,
	})
	if err != nil {
		slog.Error("answer_inline_query", "error", err)
	}
}

type trackFiles map[uint]*trackfile.File

// of returns nil if the Track has no Telegram file yet.
func (f trackFiles) of(track *library.Track) *trackfile.File {
	if track == nil {
		return nil
	}
	return f[track.ID]
}

// filesOf skips nil Tracks; on error the Tracks go without files.
func (in *Inline) filesOf(ctx context.Context, tracks ...*library.Track) trackFiles {
	ids := make([]uint, 0, len(tracks))
	for _, track := range tracks {
		if track != nil {
			ids = append(ids, track.ID)
		}
	}
	files, err := in.Files.Of(ctx, ids)
	if err != nil {
		slog.Error("find_telegram_files", "error", err)
	}
	return files
}

func (in *Inline) handleInlineFeed(ctx context.Context, queryID string) {
	c := texts(ctx)
	entries, err := in.ViewFeed.Execute(ctx, feedLimit)
	if err != nil {
		slog.Error("shared_feed", "error", err)
		in.answerInline(ctx, queryID, article("error", c.FeedFailedArticle()))
		return
	}
	if len(entries) == 0 {
		in.answerInline(ctx, queryID, article("empty-feed", c.FeedEmptyArticle()))
		return
	}

	results := make([]models.InlineQueryResult, 0, len(entries)+1)
	results = append(results, article("shared-list", c.FeedList(entries)))
	tracks := make([]*library.Track, 0, len(entries))
	for i := range entries {
		tracks = append(tracks, &entries[i].Track)
	}
	files := in.filesOf(ctx, tracks...)
	for i := range entries {
		entry := &entries[i]
		id := "shared-" + strconv.FormatUint(uint64(entry.Track.ID), 10)
		caption := c.FeedCaption(entry)
		if file := files.of(&entry.Track); file != nil {
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
	in.answerInline(ctx, queryID, results...)
}

func (in *Inline) handleInlineTop(ctx context.Context, queryID string) {
	c := texts(ctx)
	top, err := in.GetTop.Execute(ctx)
	if err != nil {
		slog.Error("get_top", "error", err)
		in.answerInline(ctx, queryID, article("error", c.TopFailedArticle()))
		return
	}
	in.answerInline(ctx, queryID, article("top", c.TopArticle(top)))
}
