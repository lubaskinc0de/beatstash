package bot

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type messageRef struct {
	chatID    int64
	messageID int
}

func (t *Telegram) reject(ctx context.Context, msg messageRef, text string) error {
	if err := t.setReaction(ctx, msg, "👎"); err != nil {
		return err
	}
	return t.replyTo(ctx, msg, text)
}

func (t *Telegram) replyTo(ctx context.Context, msg messageRef, text string) error {
	_, err := t.Bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          msg.chatID,
		Text:            text,
		ReplyParameters: &models.ReplyParameters{MessageID: msg.messageID},
	})
	if err != nil {
		return fmt.Errorf("send reply: %w", err)
	}
	return nil
}

func (t *Telegram) setReaction(ctx context.Context, msg messageRef, emoji string) error {
	_, err := t.Bot.SetMessageReaction(ctx, &bot.SetMessageReactionParams{
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
		return fmt.Errorf("set reaction: %w", err)
	}
	return nil
}

func (t *Telegram) sendText(ctx context.Context, chatID int64, text string) error {
	return t.sendKeyboard(ctx, chatID, text, nil)
}

// logUnsent logs what a handler failed to send: nobody tries it again.
func logUnsent(err error) {
	if err != nil {
		slog.Error("send", "error", err)
	}
}

func (t *Telegram) deleteMessage(ctx context.Context, chatID int64, messageID int) {
	_, err := t.Bot.DeleteMessage(ctx, &bot.DeleteMessageParams{ChatID: chatID, MessageID: messageID})
	if err != nil {
		slog.Error("delete_message", "error", err)
	}
}

func (t *Telegram) answerCallback(ctx context.Context, id, text string) {
	_, err := t.Bot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{CallbackQueryID: id, Text: text})
	if err != nil {
		slog.Error("answer_callback_query", "error", err)
	}
}

func (t *Telegram) editKeyboard(ctx context.Context, query *models.CallbackQuery, markup *models.InlineKeyboardMarkup) {
	params := &bot.EditMessageReplyMarkupParams{ReplyMarkup: markup, InlineMessageID: query.InlineMessageID}
	if msg := query.Message.Message; msg != nil {
		params.ChatID = msg.Chat.ID
		params.MessageID = msg.ID
	}
	if _, err := t.Bot.EditMessageReplyMarkup(ctx, params); err != nil {
		slog.Error("edit_reply_markup", "error", err)
	}
}

func (t *Telegram) sendKeyboard(ctx context.Context, chatID int64, text string, markup models.ReplyMarkup) error {
	_, err := t.Bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: markup,
	})
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}
	return nil
}
