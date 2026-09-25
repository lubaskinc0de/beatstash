package bot

import (
	"context"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/application"
)

var failureTexts = map[application.FailureReason]string{
	application.ReasonUnsupportedFormat: "Трек не загружен: формат не поддерживается. Подходят mp3, flac, m4a, ogg, opus и wav",
	application.ReasonCorruptFile:       "Трек не загружен: файл повреждён или это не аудио",
	application.ReasonFetchFailed:       "Трек не загружен: Telegram не отдал файл, пришлите его ещё раз",
	application.ReasonInternal:          "Трек не загружен: внутренняя ошибка, попробуйте позже",
}

type Notifier struct {
	Bot *bot.Bot
}

func (n *Notifier) Ingested(ctx context.Context, msg application.MessageRef, outcome application.IngestOutcome) {
	setReaction(ctx, n.Bot, msg, "👍")

	switch outcome {
	case application.IngestStoredInInbox:
		replyTo(ctx, n.Bot, msg, "Трек попал в Inbox: не удалось определить исполнителя или название")
	case application.IngestAlreadyExists:
		replyTo(ctx, n.Bot, msg, "Этот трек уже есть в библиотеке")
	}
}

func (n *Notifier) IngestFailed(ctx context.Context, msg application.MessageRef, reason application.FailureReason) {
	reject(ctx, n.Bot, msg, reason)
}

func reject(ctx context.Context, b *bot.Bot, msg application.MessageRef, reason application.FailureReason) {
	setReaction(ctx, b, msg, "👎")
	replyTo(ctx, b, msg, failureTexts[reason])
}

func replyTo(ctx context.Context, b *bot.Bot, msg application.MessageRef, text string) {
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          msg.ChatID,
		Text:            text,
		ReplyParameters: &models.ReplyParameters{MessageID: msg.MessageID},
	})
	if err != nil {
		slog.Error("send_reply", "error", err)
	}
}

func setReaction(ctx context.Context, b *bot.Bot, msg application.MessageRef, emoji string) {
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
