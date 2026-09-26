package bot

import (
	"context"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/i18n"
)

// Describe sets the bot description in each language and leaves only
// /start in the menu.
func Describe(ctx context.Context, b *bot.Bot, bundle *i18n.Bundle) error {
	for _, lang := range bundle.Languages() {
		c := bundle.For(lang)
		// Telegram takes a two-letter code; "" covers everyone else.
		code, _, _ := strings.Cut(string(lang), "-")
		if lang == bundle.Default() {
			code = ""
		}
		if _, err := b.SetMyDescription(ctx, &bot.SetMyDescriptionParams{Description: c.BotDescription(), LanguageCode: code}); err != nil {
			return err
		}
		_, err := b.SetMyShortDescription(ctx, &bot.SetMyShortDescriptionParams{ShortDescription: c.BotShortDescription(), LanguageCode: code})
		if err != nil {
			return err
		}
		_, err = b.SetMyCommands(ctx, &bot.SetMyCommandsParams{
			Commands:     []models.BotCommand{{Command: "start", Description: c.StartCommand()}},
			LanguageCode: code,
		})
		if err != nil {
			return err
		}
	}
	return nil
}
