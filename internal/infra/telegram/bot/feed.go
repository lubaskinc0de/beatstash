package bot

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"

	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/beatstash/internal/application/browse_shared"
	"github.com/lubaskinc0de/beatstash/internal/application/view_top"
	"github.com/lubaskinc0de/beatstash/internal/domain/library"
	"github.com/lubaskinc0de/beatstash/internal/domain/sharing"
	"github.com/lubaskinc0de/beatstash/internal/infra/telegram/i18n"
	"github.com/lubaskinc0de/beatstash/internal/infra/telegram/trackfile"
	"github.com/lubaskinc0de/beatstash/internal/infra/telegram/window"
)

// Feed is the Shared feed, its cards and the top, with Take and the file.
type Feed struct {
	Telegram        *Telegram
	ViewFeed        *browse_shared.ViewFeed
	ViewSharedTrack *browse_shared.ViewSharedTrack
	GetTop          *view_top.GetTop
	TakeTrack       *browse_shared.TakeTrack
	GetTrackAudio   *browse_shared.GetTrackAudio
	Files           *trackfile.Files
}

const feedLimit = 10

const (
	actionTake     = "tk"
	actionSendFile = "pl"
)

func (f *Feed) feedView(ctx context.Context) window.View {
	c := texts(ctx)
	rows := [][]models.InlineKeyboardButton{musicTabs(c, screenFeed)}
	tail := [][]models.InlineKeyboardButton{
		{goButton(c.TopButton(), place{screen: screenTop})},
		backRow(ctx, place{screen: screenHome}),
	}
	entries, err := f.ViewFeed.Execute(ctx, feedLimit)
	if err != nil {
		slog.Error("shared_feed", "error", err)
		return window.View{Text: c.FeedFailed(), Rows: append(rows, tail...)}
	}
	if len(entries) == 0 {
		return window.View{Text: c.FeedEmpty(), Rows: append(rows, tail...)}
	}
	for i := range entries {
		entry := &entries[i]
		rows = append(rows, []models.InlineKeyboardButton{
			goButton(c.SharedTrackButton(entry), sharedTrackPlace(entry.Track.ID)),
		})
	}
	return window.View{Text: c.FeedScreen(), Rows: append(rows, tail...)}
}

func sharedTrackPlace(sharedTrackID uint) place {
	return place{screen: screenSharedTrack, arg: strconv.FormatUint(uint64(sharedTrackID), 10)}
}

// sharedTrackView shows the feed for a Track no longer shared.
func (f *Feed) sharedTrackView(ctx context.Context, arg string) window.View {
	c := texts(ctx)
	id, _ := strconv.ParseUint(arg, 10, 64)
	sharedTrackID := uint(id)
	back := backRow(ctx, place{screen: screenFeed})
	entry, err := f.ViewSharedTrack.Execute(ctx, sharedTrackID)
	if errors.Is(err, sharing.ErrNotShared) {
		return f.feedView(ctx).WithNotice(c.NotShared())
	}
	if err != nil {
		slog.Error("view_shared_track", "error", err)
		return window.View{Text: c.TryLater(), Rows: [][]models.InlineKeyboardButton{back}}
	}
	take := models.InlineKeyboardButton{Text: c.TakeButton(), CallbackData: callbackData(actionTake, sharedTrackID), Style: styleSuccess}
	if entry.InLibrary {
		take = models.InlineKeyboardButton{Text: c.InLibraryButton(), CallbackData: take.CallbackData}
	}
	return window.View{Text: c.SharedTrackCard(entry), Rows: [][]models.InlineKeyboardButton{
		{take},
		{{Text: c.SendFileButton(), CallbackData: callbackData(actionSendFile, sharedTrackID)}},
		back,
	}}
}

func musicTabs(c i18n.Catalog, open screen) []models.InlineKeyboardButton {
	tabs := make([]models.InlineKeyboardButton, 0, 2)
	for _, tab := range []struct {
		name string
		to   screen
	}{{c.SharedTab(), screenFeed}, {c.MineTab(), screenShare}} {
		b := goButton(tab.name, place{screen: tab.to})
		if tab.to == open {
			b.Text, b.Style = c.OpenTab(tab.name), stylePrimary
		}
		tabs = append(tabs, b)
	}
	return tabs
}

func (f *Feed) handleTake(ctx context.Context, query *models.CallbackQuery, sharedTrackID uint) {
	c := texts(ctx)
	err := f.TakeTrack.Execute(ctx, sharedTrackID)
	var full *library.QuotaExceededError
	switch {
	case errors.As(err, &full):
		f.Telegram.answerCallback(ctx, query.ID, c.NoRoom(full.Usage, f.Telegram.AdminContact))
	case err == nil:
		f.Telegram.answerCallback(ctx, query.ID, c.Taken())
	case errors.Is(err, browse_shared.ErrAlreadyInLibrary):
		f.Telegram.answerCallback(ctx, query.ID, c.AlreadyInLibrary())
	case errors.Is(err, sharing.ErrNotShared):
		f.Telegram.answerCallback(ctx, query.ID, c.NotShared())
	default:
		slog.Error("take", "error", err)
		f.Telegram.answerCallback(ctx, query.ID, c.TryLater())
		return
	}
	if msg := query.Message.Message; msg != nil {
		f.Telegram.show(ctx, msg.Chat.ID, msg.ID, sharedTrackPlace(sharedTrackID), "")
	}
}

// handleSendFile sends to the private chat: the button may be in an inline
// message, where the bot cannot post.
func (f *Feed) handleSendFile(ctx context.Context, query *models.CallbackQuery, sharedTrackID uint) {
	c := texts(ctx)
	err := f.sendFile(ctx, query.From.ID, sharedTrackID)
	switch {
	case err == nil:
		f.Telegram.answerCallback(ctx, query.ID, "")
	case errors.Is(err, sharing.ErrNotShared):
		f.Telegram.answerCallback(ctx, query.ID, c.NotShared())
	case errors.Is(err, trackfile.ErrFileTooLarge):
		f.Telegram.answerCallback(ctx, query.ID, c.FileTooLarge())
	default:
		slog.Error("send_shared_file", "error", err)
		f.Telegram.answerCallback(ctx, query.ID, c.TryLater())
	}
}

// sendFile uploads the Track only once: the file serves its copies too.
func (f *Feed) sendFile(ctx context.Context, chatID int64, sharedTrackID uint) error {
	audio, err := f.GetTrackAudio.Execute(ctx, sharedTrackID)
	if err != nil {
		return err
	}
	return f.Files.SendTo(ctx, chatID, audio.Track, audio.Path)
}

func (f *Feed) topView(ctx context.Context) window.View {
	c := texts(ctx)
	back := backRow(ctx, place{screen: screenFeed})
	top, err := f.GetTop.Execute(ctx)
	if err != nil {
		slog.Error("get_top", "error", err)
		return window.View{Text: c.TopFailed(), Rows: [][]models.InlineKeyboardButton{back}}
	}
	return window.View{Text: c.Top(top), Rows: [][]models.InlineKeyboardButton{back}}
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
