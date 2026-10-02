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
	"github.com/lubaskinc0de/navidrome-tg/internal/application/search_music"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/send_listen_link"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/show_playing"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/view_top"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/i18n"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/trackfile"
)

// Inline answers inline queries and brings the files of chosen results.
type Inline struct {
	Telegram           *Telegram
	GetNowPlaying      *show_playing.GetNowPlaying
	GetRecentlyPlayed  *show_playing.GetRecentlyPlayed
	GetTrackFile       *show_playing.GetTrackFile
	GetTrackListenLink *send_listen_link.GetTrackListenLink
	GetAlbumListenLink *send_listen_link.GetAlbumListenLink
	ViewFeed           *browse_shared.ViewFeed
	GetTop             *view_top.GetTop
	SearchMusic        *search_music.SearchMusic
	Files              *trackfile.Files
	// StorageChatID gets the Tracks without a Telegram file, so inline mode
	// sends them as audio; zero turns it off.
	StorageChatID int64
}

const recentTracksLimit = 10

const searchPage = 20

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
	case "":
		in.answerHints(ctx, queryID)
	default:
		in.handleSearch(ctx, update.InlineQuery)
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
	search := article("hint-search", c.SearchHint(in.Telegram.BotName))
	empty := ""
	search.ReplyMarkup = &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{{
		{Text: c.Try(), SwitchInlineQueryCurrentChat: &empty},
	}}}
	in.answerInline(ctx, queryID, append(results, search)...)
}

func (in *Inline) handleSearch(ctx context.Context, query *models.InlineQuery) {
	c := texts(ctx)
	offset, _ := strconv.Atoi(query.Offset)
	found, err := in.SearchMusic.Execute(ctx, query.Query, offset, searchPage)
	if err != nil {
		slog.Error("search_music", "error", err)
		in.answerInline(ctx, query.ID, article("error", c.SearchFailedArticle()))
		return
	}
	if offset == 0 && len(found.Tracks) == 0 && len(found.Albums) == 0 {
		in.answerInline(ctx, query.ID, article("nothing-found", c.NothingFound()))
		return
	}

	tracks := make([]*library.Track, 0, len(found.Tracks))
	for n := range found.Tracks {
		tracks = append(tracks, &found.Tracks[n])
	}
	files := in.filesOf(ctx, tracks...)
	results := make([]models.InlineQueryResult, 0, len(found.Albums)+len(tracks))
	for _, album := range found.Albums {
		result := article(pendingResultID(albumLinkPrefix, album.TrackID, 0), c.AlbumArticle(album))
		result.ReplyMarkup = waitKeyboard()
		results = append(results, result)
	}
	hidden := false
	for _, track := range tracks {
		result, ok := in.trackResult(files, track, found.Linkable)
		if !ok {
			hidden = true
			continue
		}
		results = append(results, result)
	}
	next := ""
	if found.More {
		next = strconv.Itoa(offset + len(tracks))
	}
	in.answerPage(ctx, query.ID, next, in.unsendableButton(c, hidden), results...)
}

func (in *Inline) unsendableButton(c i18n.Catalog, hidden bool) *models.InlineQueryResultsButton {
	if !hidden {
		return nil
	}
	return &models.InlineQueryResultsButton{Text: c.UnsendableButton(), StartParameter: startUnsendable}
}

// trackResult is not ok for a Track that can be sent no way.
func (in *Inline) trackResult(files trackFiles, track *library.Track, linkable bool) (models.InlineQueryResult, bool) {
	caption := i18n.TrackCaption(track.Artist, track.Title)
	if file := files.of(track); file != nil {
		return cachedFileResult("track-"+strconv.FormatUint(uint64(track.ID), 10), file, caption), true
	}
	a := i18n.Article{Title: i18n.ResultTitle(track.Artist, track.Title), Description: track.Album, Message: caption}
	return in.unfiledResult(a, track, linkable, 0)
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

	if file := in.filesOf(ctx, track.Track).of(track.Track); file != nil {
		in.answerInline(ctx, queryID, cachedFileResult(track.ID, file, c.NowPlaying(track)))
		return
	}
	// What is playing is worth telling even without the file.
	result, ok := in.unfiledResult(c.NowPlayingArticle(track), track.Track, track.Linkable, 0)
	if !ok {
		result = article(track.ID, c.NowPlayingArticle(track))
	}
	in.answerInline(ctx, queryID, result)
}

// unfiledResult brings the file once chosen, else the Listen Link. n tells
// apart the results of one answer that stand for one Track.
func (in *Inline) unfiledResult(a i18n.Article, track *library.Track, linkable bool, n int) (models.InlineQueryResult, bool) {
	switch {
	case in.pending(track):
		return pendingResult(pendingResultID(pendingPrefix, track.ID, n), a, track), true
	case linkable && track != nil:
		return pendingResult(pendingResultID(linkPrefix, track.ID, n), a, track), true
	default:
		return nil, false
	}
}

func (in *Inline) handleRecentlyPlayed(ctx context.Context, queryID string) {
	c := texts(ctx)
	recent, err := in.GetRecentlyPlayed.Execute(ctx, recentTracksLimit)
	switch {
	case errors.Is(err, repositories.ErrNavidromeAccountNotFound):
		in.answerInline(ctx, queryID, article("no-navidrome-account", c.NoNavidromeAccount(in.Telegram.BotName)))
		return
	case err != nil:
		slog.Error("get_recently_played", "error", err)
		in.answerInline(ctx, queryID, article("error", c.HistoryFailed()))
		return
	case len(recent.Tracks) == 0:
		in.answerInline(ctx, queryID, article("nothing-played", c.HistoryEmpty()))
		return
	}
	tracks := recent.Tracks

	now := time.Now()
	results := make([]models.InlineQueryResult, 0, len(tracks)+1)
	results = append(results, article("recent-list", c.RecentList(tracks, now)))
	found := make([]*library.Track, 0, len(tracks))
	for i := range tracks {
		found = append(found, tracks[i].Track)
	}
	files := in.filesOf(ctx, found...)
	hidden := false
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
		result, ok := in.unfiledResult(a, track.Track, recent.Linkable, i)
		if !ok {
			hidden = true
			continue
		}
		results = append(results, result)
	}
	in.answerPage(ctx, queryID, "", in.unsendableButton(c, hidden), results...)
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

func (in *Inline) answerInline(ctx context.Context, inlineQueryID string, results ...models.InlineQueryResult) {
	in.answerPage(ctx, inlineQueryID, "", nil, results...)
}

// answerPage: an empty nextOffset means the last page.
func (in *Inline) answerPage(
	ctx context.Context,
	inlineQueryID, nextOffset string,
	button *models.InlineQueryResultsButton,
	results ...models.InlineQueryResult,
) {
	_, err := in.Telegram.Bot.AnswerInlineQuery(ctx, &bot.AnswerInlineQueryParams{
		InlineQueryID: inlineQueryID,
		Results:       results,
		CacheTime:     1,
		IsPersonal:    true,
		NextOffset:    nextOffset,
		Button:        button,
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
