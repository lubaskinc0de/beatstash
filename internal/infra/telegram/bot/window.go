package bot

import (
	"context"
	"log/slog"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/i18n"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/window"
)

type screen string

const (
	screenHome       screen = "home"
	screenFeed       screen = "feed"
	screenTop        screen = "top"
	screenSources    screen = "sources"
	screenProvider   screen = "provider"
	screenConnect    screen = "connect"
	screenPlan       screen = "plan"
	screenImports    screen = "imports"
	screenNavidrome  screen = "navidrome"
	screenLink       screen = "link"
	screenRegister   screen = "register"
	screenInvite     screen = "invite"
	screenHowTo      screen = "howto"
	screenListen     screen = "listen"
	screenLanguages  screen = "languages"
	screenAdmin      screen = "admin"
	screenUsers      screen = "users"
	screenUser       screen = "user"
	screenQuotas     screen = "quotas"
	screenQuota      screen = "quota"
	screenUserQuota  screen = "user_quota"
	screenShare      screen = "share"
	screenShareTrack screen = "share_track"
	screenShareAlbum screen = "share_album"
	screenUnsendable screen = "unsendable"
)

type place struct {
	screen screen
	arg    string
}

// argJoin marks onboarding screens: once done, they lead Home.
const argJoin = "join"

// Callback data never carries a text, so a button works in any language.
const (
	actionGo          = "go"
	actionLanguage    = "lang"
	actionStartImport = "zi"
	actionDisconnect  = "off"
	actionQuota       = "qt"
)

func goData(to place) string {
	if to.arg == "" {
		return actionGo + ":" + string(to.screen)
	}
	return actionGo + ":" + string(to.screen) + ":" + to.arg
}

func goButton(text string, to place) models.InlineKeyboardButton {
	return models.InlineKeyboardButton{Text: text, CallbackData: goData(to)}
}

func backRow(ctx context.Context, to place) []models.InlineKeyboardButton {
	return []models.InlineKeyboardButton{goButton(texts(ctx).Back(), to)}
}

// pageRow leafs back from page, counted from zero, and on if there are
// more; nil if neither.
func pageRow(c i18n.Catalog, page int, more bool, at func(page int) place) []models.InlineKeyboardButton {
	var row []models.InlineKeyboardButton
	if page > 0 {
		row = append(row, goButton(c.PrevPage(), at(page-1)))
	}
	if more {
		row = append(row, goButton(c.NextPage(), at(page+1)))
	}
	return row
}

// show draws the screen as the chat's window, on the message the action
// came from if that is the window; see window.Windows.Show.
func (t *Telegram) show(ctx context.Context, chatID int64, on int, at place, notice string) {
	v := t.screens.view(ctx, at.screen, at.arg).WithNotice(notice)
	t.Windows.Show(ctx, chatID, on, window.Place{Screen: string(at.screen), Arg: at.arg}, v)
}

// windowCallback is a press of a button; messageID is the pressed message.
type windowCallback struct {
	messageRef
	query *models.CallbackQuery
}

func (h *Handler) goTo(ctx context.Context, cb windowCallback, arg string) {
	name, arg, _ := strings.Cut(arg, ":")
	h.Telegram.answerCallback(ctx, cb.query.ID, "")
	h.Telegram.show(ctx, cb.chatID, cb.messageID, place{screen(name), arg}, "")
}

// handleText ignores text unless the window awaits it.
func (h *Handler) handleText(ctx context.Context, _ *bot.Bot, update *models.Update) {
	msg := update.Message
	w, err := h.Telegram.Windows.Get(ctx, msg.Chat.ID)
	if err != nil {
		slog.Error("read_window", "error", err)
		return
	}
	if w == nil {
		return
	}
	if handle := h.onText(screen(w.Screen)); handle != nil {
		handle(ctx, windowInput{
			messageRef: messageRef{chatID: msg.Chat.ID, messageID: msg.ID},
			text:       strings.TrimSpace(msg.Text),
			arg:        w.Arg,
		})
	}
}

// windowInput is a text message answering the window; arg is the window's.
type windowInput struct {
	messageRef
	text string
	arg  string
}

func isText(update *models.Update) bool {
	return update.Message != nil && update.Message.Text != "" && !strings.HasPrefix(update.Message.Text, "/")
}
