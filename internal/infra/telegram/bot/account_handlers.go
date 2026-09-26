package bot

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/navidrome"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/connect_navidrome"
)

const loginRules = "от 3 до 32 символов, латинские буквы, цифры, точка, дефис или подчёркивание"

const linkUsage = "Чтобы привязать аккаунт Navidrome, отправьте: <code>/link логин пароль</code>"

func (h *Handler) handleLink(
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

	err := h.LinkNavidromeAccount.Execute(ctx, navidrome.Credentials{Login: login, Password: password})
	if err == nil || errors.Is(err, navidrome.ErrAdminAccount) {
		h.endNavidromeLogin(ctx, chatID)
	}
	switch {
	case err == nil:
		sendText(ctx, b, chatID, fmt.Sprintf(
			"✅ Аккаунт Navidrome <b>%s</b> привязан. Теперь в Navidrome он видит только вашу личную библиотеку и общую. "+
				"Сообщение с паролем удалено",
			html.EscapeString(login),
		))
	case errors.Is(err, navidrome.ErrAdminAccount):
		sendText(ctx, b, chatID, fmt.Sprintf(
			"✅ Аккаунт Navidrome <b>%s</b> привязан. Это администратор Navidrome, поэтому он видит все библиотеки. "+
				"Сообщение с паролем удалено",
			html.EscapeString(login),
		))
	case errors.Is(err, connect_navidrome.ErrNavidromeAccountTaken):
		sendText(ctx, b, chatID, "⛔ Этот аккаунт Navidrome уже привязан к другому пользователю. Сообщение с паролем удалено")
	case errors.Is(err, navidrome.ErrInvalidCredentials):
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

func (h *Handler) handleText(
	ctx context.Context,
	b *bot.Bot,
	update *models.Update,
) {
	chatID := update.Message.Chat.ID
	awaits, err := h.Dialogs.AwaitsNavidromeLogin(ctx, chatID)
	if err != nil {
		slog.Error("read_dialog", "error", err)
		return
	}
	if awaits {
		h.registerNavidromeAccount(ctx, b, chatID, strings.TrimSpace(update.Message.Text))
	}
}

func isText(update *models.Update) bool {
	return update.Message != nil && update.Message.Text != "" && !strings.HasPrefix(update.Message.Text, "/")
}

func (h *Handler) registerNavidromeAccount(ctx context.Context, b *bot.Bot, chatID int64, login string) {
	creds, err := h.RegisterAccount.Execute(ctx, login)
	if err == nil || errors.Is(err, connect_navidrome.ErrHasNavidromeAccount) {
		h.endNavidromeLogin(ctx, chatID)
	}
	switch {
	case errors.Is(err, connect_navidrome.ErrHasNavidromeAccount):
	case err == nil:
		sendText(ctx, b, chatID, fmt.Sprintf(
			"🎧 Аккаунт Navidrome создан, входите в любом клиенте Navidrome или Subsonic\n\n"+
				"Логин: <code>%s</code>\nПароль: <tg-spoiler>%s</tg-spoiler>\n\n"+
				"Сохраните пароль: бот показывает его только один раз",
			html.EscapeString(creds.Login), html.EscapeString(creds.Password),
		))
	case errors.Is(err, connect_navidrome.ErrNavidromeLoginInvalid) && login == "":
		sendText(ctx, b, chatID, "✏️ Придумайте логин для Navidrome и отправьте его сообщением: "+loginRules)
	case errors.Is(err, connect_navidrome.ErrNavidromeLoginInvalid):
		sendText(ctx, b, chatID, "✏️ Такой логин не подойдёт. Придумайте другой: "+loginRules)
	case errors.Is(err, navidrome.ErrLoginTaken):
		sendText(ctx, b, chatID, fmt.Sprintf(
			"✏️ Логин <b>%s</b> уже занят в Navidrome. Придумайте другой и отправьте его сообщением",
			html.EscapeString(login),
		))
	default:
		slog.Error("register_navidrome_account", "error", err)
		sendText(ctx, b, chatID, "⚠️ Не удалось создать аккаунт Navidrome. Отправьте желаемый логин ещё раз чуть позже")
	}
}

func (h *Handler) endNavidromeLogin(ctx context.Context, chatID int64) {
	if err := h.Dialogs.EndNavidromeLogin(ctx, chatID); err != nil {
		slog.Error("end_navidrome_login", "error", err)
	}
}
