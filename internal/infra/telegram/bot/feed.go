package bot

import (
	"context"
	"errors"
	"log/slog"

	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/browse_shared"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/view_top"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/sharing"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/trackfile"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/window"
)

// Feed is the Shared feed and the top, with Take and the file.
type Feed struct {
	Telegram      *Telegram
	ViewFeed      *browse_shared.ViewFeed
	GetTop        *view_top.GetTop
	TakeTrack     *browse_shared.TakeTrack
	GetTrackAudio *browse_shared.GetTrackAudio
	Files         *trackfile.Files
}

const feedLimit = 10

const (
	actionTake     = "tk"
	actionSendFile = "pl"
)

func (f *Feed) feedView(ctx context.Context) window.View {
	c := texts(ctx)
	back := backRow(ctx, place{screen: screenHome})
	entries, err := f.ViewFeed.Execute(ctx, feedLimit)
	if err != nil {
		slog.Error("shared_feed", "error", err)
		return window.View{Text: c.FeedFailed(), Rows: [][]models.InlineKeyboardButton{back}}
	}
	if len(entries) == 0 {
		return window.View{Text: c.FeedEmpty(), Rows: [][]models.InlineKeyboardButton{back}}
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
	return window.View{Text: c.Feed(entries), Rows: append(rows, back)}
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
	case errors.Is(err, library.ErrAlreadyInLibrary):
		f.Telegram.answerCallback(ctx, query.ID, c.AlreadyInLibrary())
	case errors.Is(err, sharing.ErrNotShared):
		f.Telegram.answerCallback(ctx, query.ID, c.NotShared())
	default:
		slog.Error("take", "error", err)
		f.Telegram.answerCallback(ctx, query.ID, c.TryLater())
		return
	}
	if msg := query.Message.Message; msg != nil {
		f.Telegram.show(ctx, msg.Chat.ID, msg.ID, place{screen: screenFeed}, "")
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
	track, path, err := f.GetTrackAudio.Execute(ctx, sharedTrackID)
	if err != nil {
		return err
	}
	return f.Files.SendTo(ctx, chatID, track, path)
}

func (f *Feed) topView(ctx context.Context) window.View {
	c := texts(ctx)
	back := backRow(ctx, place{screen: screenHome})
	top, err := f.GetTop.Execute(ctx)
	if err != nil {
		slog.Error("get_top", "error", err)
		return window.View{Text: c.TopFailed(), Rows: [][]models.InlineKeyboardButton{back}}
	}
	return window.View{Text: c.Top(top), Rows: [][]models.InlineKeyboardButton{back}}
}
