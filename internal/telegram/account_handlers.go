package telegram

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/application"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

const loginRules = "от 3 до 32 символов, латинские буквы, цифры, точка, дефис или подчёркивание"

const linkUsage = "Чтобы привязать аккаунт Navidrome, отправьте: <code>/link логин пароль</code>"

func (h *Handler) HandleLink(
	ctx context.Context,
	b *bot.Bot,
	update *models.Update,
) {
	chatID := update.Message.Chat.ID
	_, args, _ := command(update)

	login, password, _ := strings.Cut(args, " ")
	password = strings.TrimSpace(password)
	if login == "" || password == "" {
		sendText(ctx, b, chatID, linkUsage)
		return
	}

	deleteMessage(ctx, b, chatID, update.Message.ID)

	err := h.linkNavidromeAccount.Execute(ctx, application.NavidromeCredentials{Login: login, Password: password})
	switch {
	case err == nil:
		sendText(ctx, b, chatID, fmt.Sprintf("✅ Аккаунт Navidrome <b>%s</b> привязан. Сообщение с паролем удалено", html.EscapeString(login)))
	case errors.Is(err, application.ErrNavidromeInvalidCredentials):
		sendText(ctx, b, chatID, "❌ Неверный логин или пароль Navidrome. Сообщение с паролем удалено, попробуйте ещё раз")
	default:
		slog.Error("link_navidrome_account", "error", err)
		sendText(ctx, b, chatID, "⚠️ Не удалось проверить аккаунт: Navidrome недоступен, попробуйте позже")
	}
}

func answerNoNavidromeAccount(ctx context.Context, b *bot.Bot, inlineQueryID string) {
	answerInlineArticle(
		ctx,
		b,
		inlineQueryID,
		"no-navidrome-account",
		"🔗 Привяжите аккаунт Navidrome",
		"Напишите боту: /link логин пароль",
		"🔗 "+linkUsage,
	)
}

func (h *Handler) HandleText(
	ctx context.Context,
	b *bot.Bot,
	update *models.Update,
) {
	user, ok := application.UserFromContext(ctx)
	if !ok || !user.AwaitsNavidromeLogin {
		return
	}
	h.registerNavidromeAccount(ctx, b, update.Message.Chat.ID, user, strings.TrimSpace(update.Message.Text))
}

func IsText(update *models.Update) bool {
	return update.Message != nil && update.Message.Text != "" && !strings.HasPrefix(update.Message.Text, "/")
}

func (h *Handler) registerNavidromeAccount(ctx context.Context, b *bot.Bot, chatID int64, user *domain.User, login string) {
	creds, err := h.registerAccount.Execute(ctx, user, login)
	switch {
	case err == nil:
		sendText(ctx, b, chatID, fmt.Sprintf(
			"🎧 Аккаунт Navidrome создан, входите в любом клиенте Navidrome или Subsonic\n\n"+
				"Логин: <code>%s</code>\nПароль: <tg-spoiler>%s</tg-spoiler>\n\n"+
				"Сохраните пароль: бот показывает его только один раз",
			html.EscapeString(creds.Login), html.EscapeString(creds.Password),
		))
	case errors.Is(err, application.ErrNavidromeLoginInvalid) && login == "":
		sendText(ctx, b, chatID, "✏️ Придумайте логин для Navidrome и отправьте его сообщением: "+loginRules)
	case errors.Is(err, application.ErrNavidromeLoginInvalid):
		sendText(ctx, b, chatID, "✏️ Такой логин не подойдёт. Придумайте другой: "+loginRules)
	case errors.Is(err, application.ErrNavidromeLoginTaken):
		sendText(ctx, b, chatID, fmt.Sprintf(
			"✏️ Логин <b>%s</b> уже занят в Navidrome. Придумайте другой и отправьте его сообщением",
			html.EscapeString(login),
		))
	default:
		slog.Error("register_navidrome_account", "error", err)
		sendText(ctx, b, chatID, "⚠️ Не удалось создать аккаунт Navidrome. Отправьте желаемый логин ещё раз чуть позже")
	}
}
