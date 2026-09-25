package bot

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/application"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

func (h *Handler) handleInvite(
	ctx context.Context,
	b *bot.Bot,
	update *models.Update,
) {
	chatID := update.Message.Chat.ID

	code, err := h.CreateInvite.Execute(ctx)
	if errors.Is(err, application.ErrNotAdmin) {
		sendText(ctx, b, chatID, "⛔ Приглашения выдаёт только администратор")
		return
	}
	if err != nil {
		slog.Error("create_invite", "error", err)
		sendText(ctx, b, chatID, "⚠️ Не удалось создать приглашение, попробуйте позже")
		return
	}

	username, err := botUsername(ctx, b)
	if err != nil {
		slog.Error("get_me", "error", err)
		sendText(ctx, b, chatID, "⚠️ Не удалось создать приглашение, попробуйте позже")
		return
	}
	sendText(ctx, b, chatID, fmt.Sprintf(
		"🎟 Одноразовое приглашение, действует %s:\nhttps://t.me/%s?start=%s",
		days(h.CreateInvite.TTL), username, code,
	))
}

func (h *Handler) handleStart(
	ctx context.Context,
	b *bot.Bot,
	update *models.Update,
) {
	chatID := update.Message.Chat.ID
	if user, ok := application.UserFromContext(ctx); ok {
		h.sendWelcome(ctx, b, chatID, user)
		return
	}

	_, code, _ := command(update)
	if code == "" {
		h.sendAbout(ctx, b, chatID)
		return
	}
	from := update.Message.From
	user, err := h.AcceptInvite.Execute(ctx, code, application.TelegramProfile{ID: uint64(from.ID), Username: from.Username})
	if errors.Is(err, application.ErrInviteInvalid) {
		sendText(ctx, b, chatID, "⛔ Приглашение недействительно: оно уже использовано или истекло")
		return
	}
	if err != nil {
		slog.Error("accept_invite", "error", err)
		sendText(ctx, b, chatID, "⚠️ Не удалось принять приглашение, попробуйте позже")
		return
	}
	h.sendWelcome(ctx, b, chatID, user)

	h.registerNavidromeAccount(ctx, b, chatID, user, from.Username)
}

func (h *Handler) sendWelcome(ctx context.Context, b *bot.Bot, chatID int64, user *domain.User) {
	username, err := botUsername(ctx, b)
	if err != nil {
		slog.Error("get_me", "error", err)
	}
	inviteTTL := time.Duration(0)
	if h.CreateInvite.CanInvite(user) {
		inviteTTL = h.CreateInvite.TTL
	}
	sendText(ctx, b, chatID, welcomeText(username, inviteTTL))
}

// welcomeText mentions /invite only when inviteTTL is set, that is for an Admin.
func welcomeText(botUsername string, inviteTTL time.Duration) string {
	mention := "@бот"
	if botUsername != "" {
		mention = "@" + botUsername
	}

	var b strings.Builder
	b.WriteString("👋 <b>Добро пожаловать!</b>\n\n")
	b.WriteString("Я собираю вашу личную музыкальную библиотеку в Navidrome и показываю, что вы слушаете. ")
	b.WriteString("Библиотека ваша: другие пользователи её не видят, но файлы лежат на сервере, и их видит владелец сервера.\n\n")
	b.WriteString("🎵 <b>Загрузка.</b> Пришлите аудиофайл или перешлите его из любого чата: mp3, flac, m4a, ogg, opus, wav. ")
	b.WriteString("👀 — принял, 👍 — трек в библиотеке, 👎 — объясню, что не так\n\n")
	fmt.Fprintf(&b, "🎧 <b>Сейчас играет.</b> В любом чате наберите <code>%s np</code>\n", mention)
	fmt.Fprintf(&b, "📜 <b>История.</b> <code>%s recent</code> — последние треки\n\n", mention)
	b.WriteString("🔗 <b>Share.</b> Ответьте <code>/share</code> на аудиосообщение, чтобы открыть трек или альбом всем. ")
	fmt.Fprintf(&b, "<code>/shared</code> или <code>%s shared</code> — что расшарили другие, ", mention)
	fmt.Fprintf(&b, "<code>/top</code> или <code>%s top</code> — кто больше всех делится\n\n", mention)
	b.WriteString("🔗 Уже есть аккаунт Navidrome? Привяжите его: <code>/link логин пароль</code>")
	if inviteTTL > 0 {
		fmt.Fprintf(&b, "\n\n🎟 <code>/invite</code> — одноразовое приглашение для друга, действует %s", days(inviteTTL))
	}
	return b.String()
}

// sendAbout answers a stranger's /start without revealing anybody's music.
func (h *Handler) sendAbout(ctx context.Context, b *bot.Bot, chatID int64) {
	stats, err := h.GetServiceStats.Execute(ctx)
	if err != nil {
		slog.Error("get_service_stats", "error", err)
		return
	}

	var text strings.Builder
	text.WriteString("🎧 <b>Это закрытый музыкальный сервис на Navidrome.</b>\n\n")
	text.WriteString("У каждого здесь своя музыкальная библиотека: загружать треки из Telegram, слушать их в любом клиенте Navidrome ")
	text.WriteString("и делиться находками с остальными. Попасть сюда можно только по приглашению.\n\n")
	fmt.Fprintf(&text, "👥 Пользователей: %d\n", stats.Users)
	fmt.Fprintf(&text, "🎵 Треков в общей библиотеке: %d", stats.SharedTracks)
	if h.AdminContact != "" {
		fmt.Fprintf(&text, "\n\n✉️ Попросить доступ: %s", html.EscapeString(h.AdminContact))
	}
	sendText(ctx, b, chatID, text.String())
}

func botUsername(ctx context.Context, b *bot.Bot) (string, error) {
	me, err := b.GetMe(ctx)
	if err != nil {
		return "", err
	}
	return me.Username, nil
}

func days(d time.Duration) string {
	return plural(int(d.Hours()/24), "день", "дня", "дней")
}

func plural(n int, one, few, many string) string {
	switch {
	case n%10 == 1 && n%100 != 11:
		return fmt.Sprintf("%d %s", n, one)
	case n%10 >= 2 && n%10 <= 4 && (n%100 < 12 || n%100 > 14):
		return fmt.Sprintf("%d %s", n, few)
	default:
		return fmt.Sprintf("%d %s", n, many)
	}
}
