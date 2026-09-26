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
	sendKeyboard(ctx, b, chatID, text, nil)
}

func deleteMessage(ctx context.Context, b *bot.Bot, chatID int64, messageID int) {
	_, err := b.DeleteMessage(ctx, &bot.DeleteMessageParams{ChatID: chatID, MessageID: messageID})
	if err != nil {
		slog.Error("delete_message", "error", err)
	}
}

func answerCallback(ctx context.Context, b *bot.Bot, id, text string) {
	_, err := b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{CallbackQueryID: id, Text: text})
	if err != nil {
		slog.Error("answer_callback_query", "error", err)
	}
}

func editKeyboard(ctx context.Context, b *bot.Bot, query *models.CallbackQuery, markup *models.InlineKeyboardMarkup) {
	params := &bot.EditMessageReplyMarkupParams{ReplyMarkup: markup, InlineMessageID: query.InlineMessageID}
	if msg := query.Message.Message; msg != nil {
		params.ChatID = msg.Chat.ID
		params.MessageID = msg.ID
	}
	if _, err := b.EditMessageReplyMarkup(ctx, params); err != nil {
		slog.Error("edit_reply_markup", "error", err)
	}
}

func sendKeyboard(ctx context.Context, b *bot.Bot, chatID int64, text string, markup models.ReplyMarkup) {
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: markup,
	})
	if err != nil {
		slog.Error("send_message", "error", err)
	}
}

func noKeyboard() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{}}
}
