package bot

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/store"
)

type screen string

const (
	screenHome      screen = "home"
	screenFeed      screen = "feed"
	screenTop       screen = "top"
	screenSources   screen = "sources"
	screenProvider  screen = "provider"
	screenConnect   screen = "connect"
	screenPlan      screen = "plan"
	screenImports   screen = "imports"
	screenNavidrome screen = "navidrome"
	screenLink      screen = "link"
	screenRegister  screen = "register"
	screenInvite    screen = "invite"
	screenHowTo     screen = "howto"
	screenListen    screen = "listen"
)

type place struct {
	screen screen
	arg    string
}

// argJoin marks onboarding screens: once done, they lead Home.
const argJoin = "join"

type view struct {
	text string
	rows [][]models.InlineKeyboardButton
}

func (v view) markup() *models.InlineKeyboardMarkup {
	if v.rows == nil {
		return noKeyboard()
	}
	return &models.InlineKeyboardMarkup{InlineKeyboard: v.rows}
}

// withNotice puts the outcome of the last action above the screen.
func (v view) withNotice(notice string) view {
	if notice != "" {
		v.text = notice + "\n\n" + v.text
	}
	return v
}

// Callback data never carries a text, so a button works in any language.
const (
	actionGo          = "go"
	actionLanguage    = "lang"
	actionStartImport = "zi"
	actionDisconnect  = "off"
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

func (h *Handler) render(ctx context.Context, b *bot.Bot, at place) view {
	switch at.screen {
	case screenFeed:
		return h.feedView(ctx)
	case screenTop:
		return h.topView(ctx)
	case screenSources:
		return h.sourcesView(ctx)
	case screenProvider:
		return h.providerView(ctx, at.arg)
	case screenConnect:
		return h.connectView(ctx, at.arg)
	case screenPlan:
		return h.planView(ctx, at.arg)
	case screenImports:
		return h.importsView(ctx)
	case screenNavidrome:
		return h.navidromeView(ctx)
	case screenLink:
		return h.linkView(ctx, at.arg)
	case screenRegister:
		return h.registerView(ctx)
	case screenInvite:
		return h.inviteView(ctx, b)
	case screenHowTo:
		return howToView(ctx)
	case screenListen:
		return listenView(ctx)
	default:
		return h.homeView(ctx, b)
	}
}

// textHandler returns nil for a screen that awaits no text.
func (h *Handler) textHandler(s screen) func(context.Context, *bot.Bot, windowInput) {
	switch s {
	case screenConnect:
		return h.connectProvider
	case screenLink:
		return h.linkNavidrome
	case screenRegister:
		return h.registerNavidrome
	default:
		return nil
	}
}

// openWindow also takes the buttons off the old window.
func (h *Handler) openWindow(ctx context.Context, b *bot.Bot, chatID int64, at place, notice string) {
	h.draw(ctx, b, messageRef{chatID: chatID}, at, notice)
}

// show makes the message the chat's window; an older window loses its
// buttons.
func (h *Handler) show(ctx context.Context, b *bot.Bot, msg messageRef, at place, notice string) {
	h.draw(ctx, b, msg, at, notice)
}

// showInWindow opens a window if the chat has none.
func (h *Handler) showInWindow(ctx context.Context, b *bot.Bot, chatID int64, at place, notice string) {
	msg := messageRef{chatID: chatID}
	if window := h.window(ctx, chatID); window != nil {
		msg.messageID = window.MessageID
	}
	h.draw(ctx, b, msg, at, notice)
}

// draw sends a new window when msg has no message id.
func (h *Handler) draw(ctx context.Context, b *bot.Bot, msg messageRef, at place, notice string) {
	unlock := h.chats.lock(msg.chatID)
	defer unlock()
	if old := h.window(ctx, msg.chatID); old != nil && old.MessageID != msg.messageID {
		stripKeyboard(ctx, b, msg.chatID, old.MessageID)
	}

	v := h.render(ctx, b, at).withNotice(notice)
	if msg.messageID == 0 {
		sent, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: msg.chatID, Text: v.text, ParseMode: models.ParseModeHTML,
			ReplyMarkup: v.markup(), LinkPreviewOptions: noPreview,
		})
		if err != nil {
			slog.Error("send_window", "error", err)
			return
		}
		msg.messageID = sent.ID
	} else {
		editWindow(ctx, b, msg, v)
	}
	h.remember(ctx, &store.Window{ChatID: msg.chatID, MessageID: msg.messageID, Screen: string(at.screen), Arg: at.arg, Shown: v.text})
}

// refreshImports redraws a window still on the Imports if its text changed.
// ctx carries the texts of the chat's user.
func (h *Handler) refreshImports(ctx context.Context, b *bot.Bot, chatID int64) {
	unlock := h.chats.lock(chatID)
	defer unlock()
	window := h.window(ctx, chatID)
	if window == nil || screen(window.Screen) != screenImports {
		return
	}
	v := h.importsView(ctx)
	if v.text == window.Shown {
		return
	}
	editWindow(ctx, b, messageRef{chatID: chatID, messageID: window.MessageID}, v)
	window.Shown = v.text
	h.remember(ctx, window)
}

func (h *Handler) window(ctx context.Context, chatID int64) *store.Window {
	window, err := h.Windows.Get(ctx, chatID)
	if err != nil {
		slog.Error("read_window", "error", err)
	}
	return window
}

func (h *Handler) remember(ctx context.Context, window *store.Window) {
	if err := h.Windows.Save(ctx, window); err != nil {
		slog.Error("save_window", "error", err)
	}
}

var noPreview = &models.LinkPreviewOptions{IsDisabled: bot.True()}

func editWindow(ctx context.Context, b *bot.Bot, msg messageRef, v view) {
	_, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID: msg.chatID, MessageID: msg.messageID, Text: v.text, ParseMode: models.ParseModeHTML,
		ReplyMarkup: v.markup(), LinkPreviewOptions: noPreview,
	})
	if err != nil && !strings.Contains(err.Error(), "message is not modified") {
		slog.Error("edit_window", "error", err)
	}
}

// stripKeyboard ignores a message the user deleted.
func stripKeyboard(ctx context.Context, b *bot.Bot, chatID int64, messageID int) {
	_, err := b.EditMessageReplyMarkup(ctx, &bot.EditMessageReplyMarkupParams{
		ChatID:      chatID,
		MessageID:   messageID,
		ReplyMarkup: noKeyboard(),
	})
	if err != nil && !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "not modified") {
		slog.Error("strip_window", "error", err)
	}
}

// windowCallback is a press of a button on the message msg.
type windowCallback struct {
	messageRef
	query *models.CallbackQuery
}

type windowAction func(ctx context.Context, b *bot.Bot, cb windowCallback, arg string)

func (h *Handler) windowAction(action string) (windowAction, bool) {
	switch action {
	case actionGo:
		return h.goTo, true
	case actionLanguage:
		return h.chooseLanguage, true
	case actionStartImport:
		return h.startImport, true
	case actionDisconnect:
		return h.disconnect, true
	default:
		return nil, false
	}
}

func (h *Handler) handleWindowCallback(ctx context.Context, b *bot.Bot, query *models.CallbackQuery, handle windowAction, arg string) {
	msg := query.Message.Message
	if msg == nil {
		answerCallback(ctx, b, query.ID, "")
		return
	}
	handle(ctx, b, windowCallback{messageRef: messageRef{chatID: msg.Chat.ID, messageID: msg.ID}, query: query}, arg)
}

// goTo from another message, like a notice, opens a new window and leaves
// that message as it is.
func (h *Handler) goTo(ctx context.Context, b *bot.Bot, cb windowCallback, arg string) {
	name, arg, _ := strings.Cut(arg, ":")
	to := place{screen(name), arg}
	answerCallback(ctx, b, cb.query.ID, "")
	if window := h.window(ctx, cb.chatID); window == nil || window.MessageID != cb.messageID {
		h.openWindow(ctx, b, cb.chatID, to, "")
		return
	}
	h.show(ctx, b, cb.messageRef, to, "")
}

// handleText ignores text unless the window awaits it.
func (h *Handler) handleText(ctx context.Context, b *bot.Bot, update *models.Update) {
	msg := update.Message
	window := h.window(ctx, msg.Chat.ID)
	if window == nil {
		return
	}
	if handle := h.textHandler(screen(window.Screen)); handle != nil {
		handle(ctx, b, windowInput{
			messageRef: messageRef{chatID: msg.Chat.ID, messageID: msg.ID},
			text:       strings.TrimSpace(msg.Text),
			arg:        window.Arg,
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

// chatLocks serializes redraws of a chat's window: the Poller redraws the
// Imports while the user may navigate.
type chatLocks struct {
	mu    sync.Mutex
	chats map[int64]*sync.Mutex
}

func (l *chatLocks) lock(chatID int64) (unlock func()) {
	l.mu.Lock()
	if l.chats == nil {
		l.chats = map[int64]*sync.Mutex{}
	}
	chat, ok := l.chats[chatID]
	if !ok {
		chat = &sync.Mutex{}
		l.chats[chatID] = chat
	}
	l.mu.Unlock()
	chat.Lock()
	return chat.Unlock
}
