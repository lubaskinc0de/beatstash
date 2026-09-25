package bot

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/view_top"
)

func (h *Handler) handleTop(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID

	top, err := h.GetTop.Execute(ctx)
	if err != nil {
		slog.Error("get_top", "error", err)
		sendText(ctx, b, chatID, "⚠️ Не удалось посчитать Top, попробуйте позже")
		return
	}
	sendText(ctx, b, chatID, topText(top))
}

func (h *Handler) handleInlineTop(ctx context.Context, b *bot.Bot, update *models.Update) {
	queryID := update.InlineQuery.ID

	top, err := h.GetTop.Execute(ctx)
	if err != nil {
		slog.Error("get_top", "error", err)
		answerInlineArticle(ctx, b, queryID, "error", "⚠️ Top недоступен", "Не удалось посчитать Top", "⚠️ Не удалось посчитать Top")
		return
	}
	answerInlineArticle(ctx, b, queryID, "top", "🏆 Top", "Кто больше всех расшарил и чьё чаще берут", topText(top))
}

func topText(top *view_top.Top) string {
	var b strings.Builder
	b.WriteString("🏆 <b>Top</b>")
	writeRating(&b, "🔗 Больше всех расшарил", "за всё время", top.Sharers.AllTime)
	writeRating(&b, "🔗 Больше всех расшарил", "за этот месяц", top.Sharers.ThisMonth)
	writeRating(&b, "⭐ Чаще всего берут", "за всё время", top.TakenAuthors.AllTime)
	writeRating(&b, "⭐ Чаще всего берут", "за этот месяц", top.TakenAuthors.ThisMonth)
	return b.String()
}

func writeRating(b *strings.Builder, title, period string, entries []repositories.TopEntry) {
	fmt.Fprintf(b, "\n\n<b>%s</b>, %s:", title, period)
	if len(entries) == 0 {
		b.WriteString("\nПока никого")
		return
	}
	for i, entry := range entries {
		fmt.Fprintf(b, "\n%d. %s — %d", i+1, html.EscapeString(authorName(&entry.User)), entry.Count)
	}
}
