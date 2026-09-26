package bot

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/i18n"
)

// handleStart with an invite code lets a stranger in and asks for a
// Navidrome login.
func (h *Handler) handleStart(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID
	_, code, _ := command(update)
	if _, err := h.IDs.CurrentUser(ctx); err != nil && code != "" {
		if !errors.Is(err, common.ErrNotAuthenticated) {
			slog.Error("authenticate", "error", err)
			return
		}
		h.join(ctx, b, chatID, code)
		return
	}
	h.openWindow(ctx, b, chatID, place{screen: screenHome}, "")
}

func (h *Handler) join(ctx context.Context, b *bot.Bot, chatID int64, code string) {
	c := texts(ctx)
	_, err := h.AcceptInvite.Execute(ctx, code)
	switch {
	case err == nil:
		h.openWindow(ctx, b, chatID, place{screen: screenRegister}, "")
	case errors.Is(err, access.ErrInviteInvalid):
		h.openWindow(ctx, b, chatID, place{screen: screenHome}, c.InviteInvalid())
	default:
		slog.Error("accept_invite", "error", err)
		h.openWindow(ctx, b, chatID, place{screen: screenHome}, c.TryLater())
	}
}

func (h *Handler) homeView(ctx context.Context, b *bot.Bot) view {
	c := texts(ctx)
	canInvite, err := h.CheckCanInvite.Execute(ctx)
	if errors.Is(err, common.ErrNotAuthenticated) {
		return h.strangerView(ctx)
	}
	if err != nil {
		slog.Error("can_invite", "error", err)
	}

	accounts := []models.InlineKeyboardButton{goButton(c.AccountsButton(), place{screen: screenNavidrome})}
	if canInvite {
		accounts = append(accounts, goButton(c.InviteButton(), place{screen: screenInvite}))
	}
	rows := [][]models.InlineKeyboardButton{
		{goButton(c.ListenButton(), place{screen: screenListen})},
		{goButton(c.ImportButton(), place{screen: screenSources})},
		{goButton(c.HowToButton(), place{screen: screenHowTo})},
		{goButton(c.FeedButton(), place{screen: screenFeed}), goButton(c.TopButton(), place{screen: screenTop})},
		accounts,
		{h.languageButton(c)},
	}
	return view{text: c.Home(senderName(ctx), h.botName(ctx, b)), rows: rows}
}

// strangerView tells what the service is without revealing anybody's music.
func (h *Handler) strangerView(ctx context.Context) view {
	c := texts(ctx)
	rows := [][]models.InlineKeyboardButton{{h.languageButton(c)}}
	stats, err := h.GetServiceStats.Execute(ctx)
	if err != nil {
		slog.Error("get_service_stats", "error", err)
		return view{text: c.TryLater(), rows: rows}
	}
	return view{text: c.StrangerHome(stats.Users, stats.SharedTracks, h.AdminContact), rows: rows}
}

// languageButton switches to the other language when there are two, and
// opens the list of languages otherwise.
func (h *Handler) languageButton(c i18n.Catalog) models.InlineKeyboardButton {
	langs := h.Texts.Languages()
	if len(langs) == 2 {
		other := langs[0]
		if other == c.Language() {
			other = langs[1]
		}
		return languageChoice(h.Texts.For(other))
	}
	return models.InlineKeyboardButton{Text: c.LanguagesButton(), CallbackData: actionLanguage}
}

func languageChoice(c i18n.Catalog) models.InlineKeyboardButton {
	return models.InlineKeyboardButton{Text: "🌐 " + c.LanguageName(), CallbackData: actionLanguage + ":" + string(c.Language())}
}

// languagesView uses only language callbacks, so strangers can use it:
// back re-chooses the current language.
func (h *Handler) languagesView(ctx context.Context) view {
	c := texts(ctx)
	rows := make([][]models.InlineKeyboardButton, 0, len(h.Texts.Languages())+1)
	for _, lang := range h.Texts.Languages() {
		rows = append(rows, []models.InlineKeyboardButton{languageChoice(h.Texts.For(lang))})
	}
	back := models.InlineKeyboardButton{Text: c.Back(), CallbackData: actionLanguage + ":" + string(c.Language())}
	return view{text: c.ChooseLanguage(), rows: append(rows, []models.InlineKeyboardButton{back})}
}

// chooseLanguage lists the languages when lang is empty, else sets it.
func (h *Handler) chooseLanguage(ctx context.Context, b *bot.Bot, cb windowCallback, lang string) {
	answerCallback(ctx, b, cb.query.ID, "")
	if lang == "" {
		editWindow(ctx, b, cb.messageRef, h.languagesView(ctx))
		return
	}
	c := h.Texts.For(i18n.Language(lang))
	if err := h.Users.SetLanguage(ctx, cb.query.From.ID, string(c.Language())); err != nil {
		slog.Error("save_language", "error", err)
	}
	h.show(withTexts(ctx, c), b, cb.messageRef, place{screen: screenHome}, "")
}

func senderName(ctx context.Context) string {
	s, err := senderFrom(ctx)
	if err != nil {
		return ""
	}
	if s.from.FirstName != "" {
		return s.from.FirstName
	}
	return s.from.Username
}

// inviteView gives out a new invite each time it is drawn.
func (h *Handler) inviteView(ctx context.Context, b *bot.Bot) view {
	c := texts(ctx)
	home := place{screen: screenHome}
	code, err := h.CreateInvite.Execute(ctx)
	if errors.Is(err, access.ErrNotAdmin) {
		return h.homeView(ctx, b).withNotice(c.NotAdmin())
	}
	if err != nil {
		slog.Error("create_invite", "error", err)
		return view{text: c.InviteFailed(), rows: [][]models.InlineKeyboardButton{backRow(ctx, home)}}
	}
	link := "https://t.me/" + h.botName(ctx, b) + "?start=" + code
	return view{text: c.Invite(link, h.CreateInvite.TTL), rows: [][]models.InlineKeyboardButton{
		{goButton(c.AnotherInvite(), place{screen: screenInvite})},
		backRow(ctx, home),
	}}
}

func listenView(ctx context.Context) view {
	return view{text: texts(ctx).Listen(), rows: [][]models.InlineKeyboardButton{backRow(ctx, place{screen: screenHome})}}
}

func howToView(ctx context.Context) view {
	return view{text: texts(ctx).HowTo(), rows: [][]models.InlineKeyboardButton{backRow(ctx, place{screen: screenHome})}}
}

// botNames caches the bot's username.
type botNames struct {
	mu   sync.Mutex
	name string
}

func (h *Handler) botName(ctx context.Context, b *bot.Bot) string {
	h.names.mu.Lock()
	name := h.names.name
	h.names.mu.Unlock()
	if name != "" {
		return name
	}
	me, err := b.GetMe(ctx)
	if err != nil {
		slog.Error("get_me", "error", err)
		return ""
	}
	h.names.mu.Lock()
	h.names.name = me.Username
	h.names.mu.Unlock()
	return me.Username
}
