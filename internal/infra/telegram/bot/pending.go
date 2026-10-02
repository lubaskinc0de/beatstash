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

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/listening"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/send_listen_link"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/i18n"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/trackfile"
)

// A pending result's id carries what the sent message becomes, since
// chosen_inline_result reports nothing else: its prefix, then the Track, or
// an Album by one of its Tracks.
const (
	// pendingPrefix: audio uploaded to the storage chat.
	pendingPrefix   = "pending:"
	linkPrefix      = "link:"
	albumLinkPrefix = "album:"
)

// actionWait is the button of a pending message: without a keyboard
// Telegram gives no inline_message_id to edit.
const actionWait = "wait"

// pendingResultID: n tells apart the results of one answer that stand for
// one Track.
func pendingResultID(prefix string, trackID uint, n int) string {
	return fmt.Sprintf("%s%d:%d", prefix, trackID, n)
}

func parsePendingResult(id, prefix string) (trackID uint, ok bool) {
	rest, ok := strings.CutPrefix(id, prefix)
	if !ok {
		return 0, false
	}
	track, _, _ := strings.Cut(rest, ":")
	n, err := strconv.ParseUint(track, 10, 64)
	return uint(n), err == nil
}

// pending tells whether choosing the Track can bring its file: it is on
// the bot's disk and there is a storage chat to upload it to.
func (in *Inline) pending(track *library.Track) bool {
	return track != nil && !track.Attached() && in.StorageChatID != 0
}

func pendingResult(id string, a i18n.Article, track *library.Track) *models.InlineQueryResultArticle {
	a.Message = i18n.PendingCaption(track.Artist, track.Title)
	result := article(id, a)
	result.ReplyMarkup = waitKeyboard()
	return result
}

func waitKeyboard() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{{
		{Text: "⏳", CallbackData: actionWait},
	}}}
}

func isChosenInlineResult(update *models.Update) bool {
	return update.ChosenInlineResult != nil
}

func (in *Inline) handleChosenInlineResult(ctx context.Context, _ *bot.Bot, update *models.Update) {
	chosen := update.ChosenInlineResult
	if chosen.InlineMessageID == "" {
		return
	}
	if trackID, ok := parsePendingResult(chosen.ResultID, pendingPrefix); ok {
		in.sendTrackFile(ctx, chosen.InlineMessageID, trackID)
		return
	}
	if trackID, ok := parsePendingResult(chosen.ResultID, linkPrefix); ok {
		in.sendTrackLink(ctx, chosen.InlineMessageID, trackID, false)
		return
	}
	if trackID, ok := parsePendingResult(chosen.ResultID, albumLinkPrefix); ok {
		in.sendAlbumLink(ctx, chosen.InlineMessageID, trackID)
	}
}

// sendTrackFile sends a file too large for Telegram as its Listen Link.
func (in *Inline) sendTrackFile(ctx context.Context, inlineMessageID string, trackID uint) {
	c := texts(ctx)
	track, path, err := in.GetTrackFile.Execute(ctx, trackID)
	if err != nil {
		slog.Error("get_track_file", "track_id", trackID, "error", err)
		in.editInlineText(ctx, inlineMessageID, c.NotSentNote(false))
		return
	}
	file, _, err := in.Files.For(ctx, in.StorageChatID, track, path)
	if errors.Is(err, trackfile.ErrFileTooLarge) {
		in.sendTrackLink(ctx, inlineMessageID, trackID, true)
		return
	}
	if err != nil {
		slog.Error("store_chosen_track", "track_id", trackID, "error", err)
		in.editInlineText(ctx, inlineMessageID, c.NotSentCaption(track.Artist, track.Title, false))
		return
	}
	_, err = in.Telegram.Bot.EditMessageMedia(ctx, &bot.EditMessageMediaParams{
		InlineMessageID: inlineMessageID,
		Media:           inputMedia(file, i18n.TrackCaption(track.Artist, track.Title)),
	})
	if err != nil {
		slog.Error("edit_message_media", "error", err)
	}
}

// sendTrackLink says a Track is too large to send when Listen Links are
// off.
func (in *Inline) sendTrackLink(ctx context.Context, inlineMessageID string, trackID uint, tooLarge bool) {
	c := texts(ctx)
	link, err := in.GetTrackListenLink.Execute(ctx, trackID)
	if link == nil {
		slog.Error("get_listen_link", "track_id", trackID, "error", err)
		in.editInlineText(ctx, inlineMessageID, c.NotSentNote(tooLarge))
		return
	}
	track := link.Track
	text := c.ListenLinkCaption(track.Artist, track.Title, link.URL)
	switch caption := i18n.TrackCaption(track.Artist, track.Title); {
	case errors.Is(err, listening.ErrListenLinksOff) && tooLarge:
		text = c.NotSentCaption(track.Artist, track.Title, true)
	case err != nil:
		text = linkFailure(c, caption, err)
	}
	in.editInlineText(ctx, inlineMessageID, text)
}

func (in *Inline) sendAlbumLink(ctx context.Context, inlineMessageID string, trackID uint) {
	c := texts(ctx)
	link, err := in.GetAlbumListenLink.Execute(ctx, trackID)
	if link == nil {
		slog.Error("get_album_listen_link", "track_id", trackID, "error", err)
		in.editInlineText(ctx, inlineMessageID, c.NotSentNote(false))
		return
	}
	in.editInlineText(ctx, inlineMessageID, albumLinkText(c, link, err))
}

func albumLinkText(c i18n.Catalog, link *send_listen_link.AlbumLink, err error) string {
	if err != nil {
		return linkFailure(c, i18n.AlbumCaption(link.Album.AlbumArtist, link.Album.Album), err)
	}
	return c.AlbumLinkCaption(link.Album.AlbumArtist, link.Album.Album, link.URL)
}

// linkFailure leaves the caption alone without Listen Links.
func linkFailure(c i18n.Catalog, caption string, err error) string {
	switch {
	case errors.Is(err, listening.ErrListenLinksOff), errors.Is(err, send_listen_link.ErrSingle):
		return caption
	case errors.Is(err, send_listen_link.ErrNotIndexed):
		return c.NotIndexedCaption(caption)
	default:
		slog.Error("create_listen_link", "error", err, "hint", shareHint)
		return c.LinkFailedCaption(caption)
	}
}

func linkRefusal(c i18n.Catalog, err error) string {
	if errors.Is(err, library.ErrNotKeptTrack) {
		return c.NotOwnTrack()
	}
	slog.Error("get_listen_link", "error", err)
	return c.TryLater()
}

// shareHint is the usual reason Navidrome makes no Listen Links.
const shareHint = "Navidrome makes Listen Links only with sharing on: ND_ENABLESHARING=true"

func inputMedia(file *trackfile.File, caption string) models.InputMedia {
	if file.Kind == trackfile.FileDocument {
		return &models.InputMediaDocument{Media: file.ID, Caption: caption, ParseMode: models.ParseModeHTML}
	}
	return &models.InputMediaAudio{Media: file.ID, Caption: caption, ParseMode: models.ParseModeHTML}
}

func (in *Inline) editInlineText(ctx context.Context, inlineMessageID, text string) {
	_, err := in.Telegram.Bot.EditMessageText(ctx, &bot.EditMessageTextParams{
		InlineMessageID: inlineMessageID,
		Text:            text,
		ParseMode:       models.ParseModeHTML,
	})
	if err != nil {
		slog.Error("edit_inline_message", "error", err)
	}
}
