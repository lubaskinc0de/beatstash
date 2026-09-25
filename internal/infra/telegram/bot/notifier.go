package bot

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/providers"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/ingest_track"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

var failureTexts = map[providers.FailureReason]string{
	providers.ReasonUnsupportedFormat: "Трек не загружен: формат не поддерживается. Подходят mp3, flac, m4a, ogg, opus и wav",
	providers.ReasonCorruptFile:       "Трек не загружен: файл повреждён или это не аудио",
	providers.ReasonFetchFailed:       "Трек не загружен: Telegram не отдал файл, пришлите его ещё раз",
	providers.ReasonInternal:          "Трек не загружен: внутренняя ошибка, попробуйте позже",
}

type Notifier struct {
	Bot              *bot.Bot
	ProgressInterval time.Duration

	edits progressEdits
}

func (n *Notifier) Ingested(ctx context.Context, msg common.MessageRef, outcome ingest_track.IngestOutcome) {
	setReaction(ctx, n.Bot, msg, "👍")

	switch outcome {
	case ingest_track.IngestStoredInInbox:
		replyTo(ctx, n.Bot, msg, "Трек попал в Inbox: не удалось определить исполнителя или название")
	case ingest_track.IngestAlreadyExists:
		replyTo(ctx, n.Bot, msg, "Этот трек уже есть в библиотеке")
	}
}

func (n *Notifier) IngestFailed(ctx context.Context, msg common.MessageRef, reason providers.FailureReason) {
	reject(ctx, n.Bot, msg, failureTexts[reason])
}

func reject(ctx context.Context, b *bot.Bot, msg common.MessageRef, text string) {
	setReaction(ctx, b, msg, "👎")
	replyTo(ctx, b, msg, text)
}

// maxFailedListed keeps a summary within Telegram's 4096 characters.
const maxFailedListed = 40

func (n *Notifier) Announce(ctx context.Context, batch *domain.IngestBatch) (int, error) {
	params := &bot.SendMessageParams{ChatID: batch.ChatID, Text: batchText(batch, repositories.BatchProgress{}), ParseMode: models.ParseModeHTML}
	msg, err := n.Bot.SendMessage(ctx, params)
	if err != nil {
		return 0, err
	}
	return msg.ID, nil
}

func (n *Notifier) Progress(ctx context.Context, batch *domain.IngestBatch, progress repositories.BatchProgress) {
	n.edits.progress(ctx, batch, progress, n.ProgressInterval, func(progress repositories.BatchProgress) {
		n.edit(ctx, batch, batchText(batch, progress))
	})
}

func (n *Notifier) Withdraw(ctx context.Context, batch *domain.IngestBatch) {
	deleteMessage(ctx, n.Bot, batch.ChatID, batch.MessageID)
}

func (n *Notifier) Finished(ctx context.Context, batch *domain.IngestBatch, progress repositories.BatchProgress, failed []string) {
	var text strings.Builder
	fmt.Fprintf(&text, "✅ %s: %d из %s в библиотеке", importName, progress.Done, plural(batch.Total, "трека", "треков", "треков"))
	if len(failed) > 0 {
		fmt.Fprintf(&text, "\n\n❌ Не удалось загрузить (%d):", len(failed))
		for _, name := range failed[:min(len(failed), maxFailedListed)] {
			fmt.Fprintf(&text, "\n• %s", html.EscapeString(name))
		}
		if len(failed) > maxFailedListed {
			fmt.Fprintf(&text, "\n…и ещё %d", len(failed)-maxFailedListed)
		}
	}
	n.edits.finished(batch.ID, func() { n.edit(ctx, batch, text.String()) })
}

func (n *Notifier) edit(ctx context.Context, batch *domain.IngestBatch, text string) {
	_, err := n.Bot.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    batch.ChatID,
		MessageID: batch.MessageID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	})
	if err != nil {
		slog.Error("edit_progress_message", "batch_id", batch.ID, "error", err)
	}
}

func batchText(batch *domain.IngestBatch, progress repositories.BatchProgress) string {
	text := fmt.Sprintf("⏳ %s: готово %d из %d", importName, progress.Done+progress.Failed, batch.Total)
	if progress.Failed > 0 {
		text += fmt.Sprintf(", ошибок %d", progress.Failed)
	}
	return text
}

const importName = "Import из Звука"

func replyTo(ctx context.Context, b *bot.Bot, msg common.MessageRef, text string) {
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          msg.ChatID,
		Text:            text,
		ReplyParameters: &models.ReplyParameters{MessageID: msg.MessageID},
	})
	if err != nil {
		slog.Error("send_reply", "error", err)
	}
}

func setReaction(ctx context.Context, b *bot.Bot, msg common.MessageRef, emoji string) {
	_, err := b.SetMessageReaction(ctx, &bot.SetMessageReactionParams{
		ChatID:    msg.ChatID,
		MessageID: msg.MessageID,
		Reaction: []models.ReactionType{
			{
				Type: models.ReactionTypeTypeEmoji,
				ReactionTypeEmoji: &models.ReactionTypeEmoji{
					Emoji: emoji,
				},
			},
		},
	})
	if err != nil {
		slog.Error("set_reaction", "error", err)
	}
}

func sendText(ctx context.Context, b *bot.Bot, chatID int64, text string) {
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	})
	if err != nil {
		slog.Error("send_message", "error", err)
	}
}

func deleteMessage(ctx context.Context, b *bot.Bot, chatID int64, messageID int) {
	_, err := b.DeleteMessage(ctx, &bot.DeleteMessageParams{ChatID: chatID, MessageID: messageID})
	if err != nil {
		slog.Error("delete_message", "error", err)
	}
}

func (n *Notifier) ProviderAccountInvalid(ctx context.Context, user *domain.User, _ domain.ProviderName) {
	sendText(ctx, n.Bot, int64(user.TelegramID),
		"🔌 Звук перестал принимать токен, поэтому новые лайки и треки плейлистов больше не подтягиваются. "+
			"Подключите Звук заново: <code>/zvuk токен</code>")
}
