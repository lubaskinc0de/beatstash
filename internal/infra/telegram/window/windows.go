package window

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"gorm.io/gorm"
)

// The message a window is shown on, besides a message id.
const (
	// New sends a new window.
	New = 0
	// Current stands for the chat's window, whichever message it is.
	Current = -1
)

// drawWait is how often a redraw waiting for another looks at the lease.
const drawWait = 50 * time.Millisecond

type Windows struct {
	DB  *gorm.DB
	Bot *bot.Bot
	// LeaseTTL outlasts a redraw: an older lease was left by an instance
	// that died amid one.
	LeaseTTL time.Duration
}

// Get returns nil if the chat has no window.
func (w *Windows) Get(ctx context.Context, chatID int64) (*Window, error) {
	var window Window
	err := w.DB.WithContext(ctx).Where("chat_id = ?", chatID).Take(&window).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &window, nil
}

// Show makes the view the chat's window. It edits the window in place if on
// is the window and nothing is below it; otherwise it sends a new window
// and then removes the old one. While another redraw of the chat goes on,
// Show waits for it as long as ctx lives.
func (w *Windows) Show(ctx context.Context, chatID int64, on int, at Place, v View) {
	window, err := w.take(ctx, chatID)
	if err != nil {
		slog.Error("take_window", "error", err)
		return
	}
	if on == Current {
		on = window.MessageID
	}
	if on != window.MessageID || window.Below > 0 {
		on = New
	}
	if on == New {
		sent, err := w.Bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID, Text: v.Text, ParseMode: models.ParseModeHTML,
			ReplyMarkup: v.Markup(), LinkPreviewOptions: noPreview,
		})
		if err != nil {
			slog.Error("send_window", "error", err)
			w.release(ctx, chatID, *window.DrawingUntil)
			return
		}
		on = sent.ID
		if window.MessageID != 0 {
			w.remove(ctx, chatID, window.MessageID)
		}
	} else {
		w.edit(ctx, chatID, on, v)
	}
	w.save(ctx, window, Window{ChatID: chatID, MessageID: on, Screen: at.Screen, Arg: at.Arg, Shown: v.Text})
}

// On lists the chats whose window is on the screen.
func (w *Windows) On(ctx context.Context, screen string) ([]int64, error) {
	var chatIDs []int64
	err := w.DB.WithContext(ctx).Model(&Window{}).Where("screen = ?", screen).Order("chat_id").Pluck("chat_id", &chatIDs).Error
	return chatIDs, err
}

// Refresh redraws the windows on the screen whose view changed. A window
// another redraw holds, or one that left the screen since the views were
// made, is left to the next refresh.
func (w *Windows) Refresh(ctx context.Context, screen string, views map[int64]View) error {
	if len(views) == 0 {
		return nil
	}
	values := make([]string, 0, len(views))
	args := []any{w.LeaseTTL.Milliseconds()}
	for chatID, v := range views {
		values = append(values, "(?::bigint, ?::text)")
		args = append(args, chatID, v.Text)
	}
	var windows []Window
	err := w.DB.WithContext(ctx).Raw(`
		UPDATE telegram_windows SET drawing_until = now() + ? * interval '1 millisecond'
		FROM (VALUES `+strings.Join(values, ", ")+`) AS v(chat_id, shown)
		WHERE telegram_windows.chat_id = v.chat_id AND screen = ? AND telegram_windows.shown <> v.shown
		AND (drawing_until IS NULL OR drawing_until < now())
		RETURNING telegram_windows.*`,
		append(args, screen)...,
	).Scan(&windows).Error
	if err != nil || len(windows) == 0 {
		return err
	}

	values = values[:0]
	args = args[:0]
	for _, window := range windows {
		v := views[window.ChatID]
		w.edit(ctx, window.ChatID, window.MessageID, v)
		values = append(values, "(?::bigint, ?::text)")
		args = append(args, window.ChatID, v.Text)
	}
	// One lease covers them all: they were taken by one statement.
	return w.DB.WithContext(context.WithoutCancel(ctx)).Exec(`
		UPDATE telegram_windows SET shown = v.shown, drawing_until = NULL
		FROM (VALUES `+strings.Join(values, ", ")+`) AS v(chat_id, shown)
		WHERE telegram_windows.chat_id = v.chat_id AND drawing_until = ?`,
		append(args, *windows[0].DrawingUntil)...,
	).Error
}

// Arrived notes a message of the chat; one older than the window, like an
// edited one, is not below it.
func (w *Windows) Arrived(ctx context.Context, chatID int64, messageID int) error {
	return w.DB.WithContext(ctx).Model(&Window{}).
		Where("chat_id = ? AND message_id < ?", chatID, messageID).
		Update("below", gorm.Expr("below + 1")).Error
}

func (w *Windows) Deleted(ctx context.Context, chatID int64, messageID int) error {
	return w.DB.WithContext(ctx).Model(&Window{}).
		Where("chat_id = ? AND message_id < ? AND below > 0", chatID, messageID).
		Update("below", gorm.Expr("below - 1")).Error
}

// take waits for the chat's window to be free and leases it. A chat that
// has no window yet gets a row for one, so its first draw is leased too.
func (w *Windows) take(ctx context.Context, chatID int64) (*Window, error) {
	for {
		var windows []Window
		err := w.DB.WithContext(ctx).Raw(`
			INSERT INTO telegram_windows (chat_id, message_id, screen, arg, shown, below, drawing_until)
			VALUES (@chat, 0, '', '', '', 0, now() + @ttl * interval '1 millisecond')
			ON CONFLICT (chat_id) DO UPDATE SET drawing_until = EXCLUDED.drawing_until
			WHERE telegram_windows.drawing_until IS NULL OR telegram_windows.drawing_until < now()
			RETURNING *`,
			sql.Named("chat", chatID), sql.Named("ttl", w.LeaseTTL.Milliseconds()),
		).Scan(&windows).Error
		if err != nil {
			return nil, err
		}
		if len(windows) > 0 {
			return &windows[0], nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(drawWait):
		}
	}
}

// save keeps the count of messages below while the window stays the same
// message: they may arrive while it is redrawn. A lease that ran out saves
// nothing: the window may have been redrawn since.
func (w *Windows) save(ctx context.Context, old *Window, window Window) {
	result := w.DB.WithContext(context.WithoutCancel(ctx)).Exec(`
		UPDATE telegram_windows SET message_id = @message, screen = @screen, arg = @arg, shown = @shown,
		below = CASE WHEN message_id = @message THEN below ELSE 0 END, drawing_until = NULL
		WHERE chat_id = @chat AND drawing_until = @token`,
		sql.Named("message", window.MessageID), sql.Named("screen", window.Screen), sql.Named("arg", window.Arg),
		sql.Named("shown", window.Shown), sql.Named("chat", window.ChatID), sql.Named("token", *old.DrawingUntil),
	)
	switch {
	case result.Error != nil:
		slog.Error("save_window", "error", result.Error)
	case result.RowsAffected == 0:
		slog.Error("save_window", "error", fmt.Errorf("lease of chat %d ran out", window.ChatID))
	}
}

// release frees the window even for an instance that is stopping: it
// would wait out the lease otherwise.
func (w *Windows) release(ctx context.Context, chatID int64, token time.Time) {
	err := w.DB.WithContext(context.WithoutCancel(ctx)).Model(&Window{}).
		Where("chat_id = ? AND drawing_until = ?", chatID, token).
		Update("drawing_until", nil).Error
	if err != nil {
		slog.Error("release_window", "error", err)
	}
}

func (w *Windows) edit(ctx context.Context, chatID int64, messageID int, v View) {
	_, err := w.Bot.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID: chatID, MessageID: messageID, Text: v.Text, ParseMode: models.ParseModeHTML,
		ReplyMarkup: v.Markup(), LinkPreviewOptions: noPreview,
	})
	if err != nil && !strings.Contains(err.Error(), "message is not modified") {
		slog.Error("edit_window", "error", err)
	}
}

// remove deletes the old window, or takes its buttons off if Telegram keeps
// it, as it does with a message older than two days.
func (w *Windows) remove(ctx context.Context, chatID int64, messageID int) {
	_, err := w.Bot.DeleteMessage(ctx, &bot.DeleteMessageParams{ChatID: chatID, MessageID: messageID})
	if err != nil && !strings.Contains(err.Error(), "not found") {
		w.stripKeyboard(ctx, chatID, messageID)
	}
}

// stripKeyboard ignores a message the user deleted.
func (w *Windows) stripKeyboard(ctx context.Context, chatID int64, messageID int) {
	_, err := w.Bot.EditMessageReplyMarkup(ctx, &bot.EditMessageReplyMarkupParams{
		ChatID:      chatID,
		MessageID:   messageID,
		ReplyMarkup: NoKeyboard(),
	})
	if err != nil && !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "not modified") {
		slog.Error("strip_window", "error", err)
	}
}
