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

	"github.com/lubaskinc0de/beatstash/internal/application/common/listening"
	"github.com/lubaskinc0de/beatstash/internal/application/common/navidrome"
	"github.com/lubaskinc0de/beatstash/internal/application/send_listen_link"
	"github.com/lubaskinc0de/beatstash/internal/application/show_playing"
	"github.com/lubaskinc0de/beatstash/internal/domain/library"
	"github.com/lubaskinc0de/beatstash/internal/domain/sharing"
	"github.com/lubaskinc0de/beatstash/internal/infra/telegram/i18n"
	"github.com/lubaskinc0de/beatstash/internal/infra/telegram/trackfile"
)

// A pending result's id carries what the sent message becomes, since
// chosen_inline_result reports nothing else: its prefix, then the Track, or
// an Album by one of its Tracks, then its tag.
const (
	// pendingPrefix: the Track's audio, uploaded to the storage chat if it
	// has no file yet.
	pendingPrefix   = "pending:"
	linkPrefix      = "link:"
	albumLinkPrefix = "album:"
	// sharedPrefix: audio of a feed Track, signed with who shared it.
	sharedPrefix = "shared:"
)

// actionWait is the button of a pending message: without a keyboard
// Telegram gives no inline_message_id to edit.
const actionWait = "wait"

// pendingResultID: the tag tells apart the results of one answer that
// stand for one Track, or keeps what np showed.
func pendingResultID(prefix string, trackID uint, tag string) string {
	return fmt.Sprintf("%s%d:%s", prefix, trackID, tag)
}

func parsePendingResult(id, prefix string) (trackID uint, tag string, ok bool) {
	rest, ok := strings.CutPrefix(id, prefix)
	if !ok {
		return 0, "", false
	}
	track, tag, _ := strings.Cut(rest, ":")
	n, err := strconv.ParseUint(track, 10, 64)
	return uint(n), tag, err == nil
}

// playingTag keeps what np showed, within the 64 bytes of a result id:
// once the result is chosen, something else may be playing.
func playingTag(track *show_playing.NowPlaying) string {
	paused := 0
	if track.Paused() {
		paused = 1
	}
	return fmt.Sprintf("np:%d:%d:%d", track.PositionMs/1000, track.Duration, paused)
}

// playingOf is what np showed of the Track, by the tag; nil for a tag of
// another result.
func playingOf(tag string, track *library.Track) *show_playing.NowPlaying {
	var position, duration, paused int
	if _, err := fmt.Sscanf(tag, "np:%d:%d:%d", &position, &duration, &paused); err != nil {
		return nil
	}
	state := "playing"
	if paused == 1 {
		state = navidrome.StatePaused
	}
	return &show_playing.NowPlaying{
		PlayingTrack: navidrome.PlayingTrack{
			Track:      navidrome.Track{Artist: track.Artist, Title: track.Title, Album: track.Album, Duration: duration},
			PositionMs: position * 1000,
			State:      state,
		},
		Track: track,
	}
}

func chosenCaption(c i18n.Catalog, track *library.Track, tag string) string {
	if playing := playingOf(tag, track); playing != nil {
		return c.NowPlaying(playing)
	}
	return i18n.TrackCaption(track.Artist, track.Title)
}

// pending tells whether choosing the Track can bring its file: it has one
// already, or it is on the bot's disk and there is a storage chat to upload
// it to.
func (in *Inline) pending(files trackFiles, track *library.Track) bool {
	return track != nil && (files.of(track) != nil || !track.Attached() && in.StorageChatID != 0)
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
	if trackID, tag, ok := parsePendingResult(chosen.ResultID, pendingPrefix); ok {
		in.sendTrackFile(ctx, chosen.InlineMessageID, trackID, tag)
		return
	}
	if trackID, tag, ok := parsePendingResult(chosen.ResultID, linkPrefix); ok {
		in.sendTrackLink(ctx, chosen.InlineMessageID, trackID, tag, false)
		return
	}
	if trackID, _, ok := parsePendingResult(chosen.ResultID, sharedPrefix); ok {
		in.sendFeedTrack(ctx, chosen.InlineMessageID, trackID)
		return
	}
	if trackID, _, ok := parsePendingResult(chosen.ResultID, albumLinkPrefix); ok {
		in.sendAlbumLink(ctx, chosen.InlineMessageID, trackID)
	}
}

func (in *Inline) sendFeedTrack(ctx context.Context, inlineMessageID string, sharedTrackID uint) {
	c := texts(ctx)
	audio, err := in.GetTrackAudio.Execute(ctx, sharedTrackID)
	if err != nil {
		if !errors.Is(err, sharing.ErrNotShared) {
			slog.Error("get_track_audio", "track_id", sharedTrackID, "error", err)
		}
		in.editInlineText(ctx, inlineMessageID, c.NotSentNote(false))
		return
	}
	caption := c.FeedCaption(audio.Track, &audio.Author)
	if tooLarge := in.editToFile(ctx, inlineMessageID, audio.Track, audio.Path, caption); tooLarge {
		in.editInlineText(ctx, inlineMessageID, c.NotSentCaption(caption, true))
	}
}

// sendTrackFile sends a file too large for Telegram as its Listen Link.
func (in *Inline) sendTrackFile(ctx context.Context, inlineMessageID string, trackID uint, tag string) {
	c := texts(ctx)
	track, path, err := in.GetTrackFile.Execute(ctx, trackID)
	if err != nil {
		slog.Error("get_track_file", "track_id", trackID, "error", err)
		in.editInlineText(ctx, inlineMessageID, c.NotSentNote(false))
		return
	}
	if tooLarge := in.editToFile(ctx, inlineMessageID, track, path, chosenCaption(c, track, tag)); tooLarge {
		in.sendTrackLink(ctx, inlineMessageID, trackID, tag, true)
	}
}

// editToFile turns the pending message into the Track's file under the
// caption, or the caption with a note if the file cannot be sent. A file
// too large for Telegram is left to the caller.
func (in *Inline) editToFile(ctx context.Context, inlineMessageID string, track *library.Track, path, caption string) (tooLarge bool) {
	file, _, err := in.Files.For(ctx, in.StorageChatID, track, path)
	if errors.Is(err, trackfile.ErrFileTooLarge) {
		return true
	}
	if err != nil {
		slog.Error("store_chosen_track", "track_id", track.ID, "error", err)
		in.editInlineText(ctx, inlineMessageID, texts(ctx).NotSentCaption(caption, false))
		return false
	}
	_, err = in.Telegram.Bot.EditMessageMedia(ctx, &bot.EditMessageMediaParams{
		InlineMessageID: inlineMessageID,
		Media:           inputMedia(file, caption),
	})
	if err != nil {
		slog.Error("edit_message_media", "error", err)
	}
	return false
}

// sendTrackLink says a Track is too large to send when Listen Links are
// off.
func (in *Inline) sendTrackLink(ctx context.Context, inlineMessageID string, trackID uint, tag string, tooLarge bool) {
	c := texts(ctx)
	link, err := in.GetTrackListenLink.Execute(ctx, trackID)
	if link == nil {
		slog.Error("get_listen_link", "track_id", trackID, "error", err)
		in.editInlineText(ctx, inlineMessageID, c.NotSentNote(tooLarge))
		return
	}
	caption := chosenCaption(c, link.Track, tag)
	text := c.ListenLinkCaption(caption, link.URL)
	switch {
	case errors.Is(err, listening.ErrListenLinksOff) && tooLarge:
		text = c.NotSentCaption(caption, true)
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
