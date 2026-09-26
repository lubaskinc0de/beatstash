package bot

import (
	"context"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type messageRef struct {
	chatID    int64
	messageID int
}

func reject(ctx context.Context, b *bot.Bot, msg messageRef, text string) {
	setReaction(ctx, b, msg, "👎")
	replyTo(ctx, b, msg, text)
}

func replyTo(ctx context.Context, b *bot.Bot, msg messageRef, text string) {
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          msg.chatID,
		Text:            text,
		ReplyParameters: &models.ReplyParameters{MessageID: msg.messageID},
	})
	if err != nil {
		slog.Error("send_reply", "error", err)
	}
}

func setReaction(ctx context.Context, b *bot.Bot, msg messageRef, emoji string) {
	_, err := b.SetMessageReaction(ctx, &bot.SetMessageReactionParams{
		ChatID:    msg.chatID,
		MessageID: msg.messageID,
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
