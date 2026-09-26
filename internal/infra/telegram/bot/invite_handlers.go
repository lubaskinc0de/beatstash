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

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
)

func (h *Handler) handleInvite(
	ctx context.Context,
	b *bot.Bot,
	update *models.Update,
) {
	chatID := update.Message.Chat.ID

	code, err := h.CreateInvite.Execute(ctx)
	if errors.Is(err, access.ErrNotAdmin) {
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
	canInvite, err := h.CheckCanInvite.Execute(ctx)
	switch {
	case err == nil:
		h.sendWelcome(ctx, b, chatID, canInvite)
		return
	case !errors.Is(err, common.ErrNotAuthenticated):
		slog.Error("can_invite", "error", err)
		return
	}

	_, code, _ := command(update)
	if code == "" {
		h.sendAbout(ctx, b, chatID)
		return
	}
	user, err := h.AcceptInvite.Execute(ctx, code)
	if errors.Is(err, access.ErrInviteInvalid) {
		sendText(ctx, b, chatID, "⛔ Приглашение недействительно: оно уже использовано или истекло")
		return
	}
	if err != nil {
		slog.Error("accept_invite", "error", err)
		sendText(ctx, b, chatID, "⚠️ Не удалось принять приглашение, попробуйте позже")
		return
	}
	if canInvite, err = h.CheckCanInvite.Execute(ctx); err != nil {
		slog.Error("can_invite", "error", err)
	}
	h.sendWelcome(ctx, b, chatID, canInvite)

	if err := h.Dialogs.AwaitNavidromeLogin(ctx, chatID); err != nil {
		slog.Error("await_navidrome_login", "error", err)
	}
	h.registerNavidromeAccount(ctx, b, chatID, user.Username)
}

func (h *Handler) sendWelcome(ctx context.Context, b *bot.Bot, chatID int64, canInvite bool) {
	username, err := botUsername(ctx, b)
	if err != nil {
		slog.Error("get_me", "error", err)
	}
	inviteTTL := time.Duration(0)
	if canInvite {
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
	b.WriteString("🟣 <b>Звук.</b> Подключите аккаунт: <code>/zvuk токен</code>. " + zvukTokenHowTo + ". ")
	b.WriteString("Потом <code>/zvuk_import</code> перенесёт ")
	b.WriteString("все лайки, альбомы и плейлисты: лайки станут звёздами, плейлисты — плейлистами Navidrome, ")
	b.WriteString("а новое будет подтягиваться само. <code>/zvuk_off</code> — отключить\n\n")
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
	text.WriteString("У каждого здесь своя музыкальная библиотека: загружать треки из Telegram и Звука, слушать их в любом клиенте Navidrome ")
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
