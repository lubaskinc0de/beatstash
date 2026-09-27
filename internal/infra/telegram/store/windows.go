package store

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Window is the one message of a chat the bot redraws.
type Window struct {
	ChatID    int64  `gorm:"primaryKey;autoIncrement:false"`
	MessageID int    `gorm:"not null"`
	Screen    string `gorm:"not null;index"`
	// Arg is what the screen is about, e.g. the Provider.
	Arg string `gorm:"not null"`
	// Shown lets the Poller edit only on a change.
	Shown string `gorm:"not null"`
	// Below counts the messages under the window, the user's and the bot's
	// alike: an edit of a window with any is not seen.
	Below int `gorm:"not null;default:0"`
}

func (Window) TableName() string {
	return "telegram_windows"
}

type Windows struct {
	DB *gorm.DB
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

// Save keeps the count of messages below while the window stays the same
// message: they may arrive while it is redrawn.
func (w *Windows) Save(ctx context.Context, window *Window) error {
	return w.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "chat_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"message_id": gorm.Expr("excluded.message_id"),
			"screen":     gorm.Expr("excluded.screen"),
			"arg":        gorm.Expr("excluded.arg"),
			"shown":      gorm.Expr("excluded.shown"),
			"below": gorm.Expr("CASE WHEN telegram_windows.message_id = excluded.message_id " +
				"THEN telegram_windows.below ELSE 0 END"),
		}),
	}).Omit("below").Create(window).Error
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

func (w *Windows) OnScreen(ctx context.Context, screen string) ([]Window, error) {
	var windows []Window
	err := w.DB.WithContext(ctx).Where("screen = ?", screen).Order("chat_id").Find(&windows).Error
	return windows, err
}
