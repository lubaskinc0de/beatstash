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

func (w *Windows) Save(ctx context.Context, window *Window) error {
	return w.DB.WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).Create(window).Error
}

func (w *Windows) OnScreen(ctx context.Context, screen string) ([]Window, error) {
	var windows []Window
	err := w.DB.WithContext(ctx).Where("screen = ?", screen).Order("chat_id").Find(&windows).Error
	return windows, err
}
