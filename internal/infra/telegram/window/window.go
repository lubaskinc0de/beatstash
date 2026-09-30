// Package window manages each chat's redrawable message and coordinates
// redraws across instances with a database lease.
package window

import (
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type Window struct {
	ChatID int64 `gorm:"primaryKey;autoIncrement:false"`
	// MessageID is zero until the window is first drawn.
	MessageID int    `gorm:"not null"`
	Screen    string `gorm:"not null;index"`
	// Arg is what the screen is about, e.g. the Provider.
	Arg string `gorm:"not null"`
	// Shown lets a refresh edit only on a change.
	Shown string `gorm:"not null"`
	// Below counts the messages under the window, the user's and the bot's
	// alike: an edit of a window with any is not seen.
	Below int `gorm:"not null;default:0"`
	// DrawingUntil is the lease of whoever draws the window; its value is
	// their token to save it.
	DrawingUntil *time.Time
}

func (Window) TableName() string {
	return "telegram_windows"
}

type Place struct {
	Screen string
	Arg    string
}

type View struct {
	Text string
	Rows [][]models.InlineKeyboardButton
}

func (v View) Markup() *models.InlineKeyboardMarkup {
	if v.Rows == nil {
		return NoKeyboard()
	}
	return &models.InlineKeyboardMarkup{InlineKeyboard: v.Rows}
}

// WithNotice puts the outcome of the last action above the screen.
func (v View) WithNotice(notice string) View {
	if notice != "" {
		v.Text = notice + "\n\n" + v.Text
	}
	return v
}

func NoKeyboard() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{}}
}

var noPreview = &models.LinkPreviewOptions{IsDisabled: bot.True()}
