package bot

import (
	"context"
	"errors"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/greet_stranger"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/invite_friend"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/join_by_invite"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/i18n"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/window"
)

// Home is the home screen, the invites and the language.
type Home struct {
	Telegram        *Telegram
	IDs             common.IDProvider
	Users           *Users
	CheckCanInvite  *invite_friend.CheckCanInvite
	CreateInvite    *invite_friend.CreateInvite
	AcceptInvite    *join_by_invite.AcceptInvite
	GetServiceStats *greet_stranger.GetServiceStats
	AdminContact    string
}

// handleStart with an invite code lets a stranger in and asks for a
// Navidrome login.
func (h *Home) handleStart(ctx context.Context, _ *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID
	_, code, _ := command(update)
	if _, err := h.IDs.CurrentUser(ctx); err != nil && code != "" {
		if !errors.Is(err, common.ErrNotAuthenticated) {
			slog.Error("authenticate", "error", err)
			return
		}
		h.join(ctx, chatID, code)
		return
	}
	h.Telegram.show(ctx, chatID, window.New, place{screen: screenHome}, "")
}

func (h *Home) join(ctx context.Context, chatID int64, code string) {
	c := texts(ctx)
	_, err := h.AcceptInvite.Execute(ctx, code)
	switch {
	case err == nil:
		h.Telegram.show(ctx, chatID, window.New, place{screen: screenRegister}, "")
	case errors.Is(err, access.ErrInviteInvalid):
		h.Telegram.show(ctx, chatID, window.New, place{screen: screenHome}, c.InviteInvalid())
	default:
		slog.Error("accept_invite", "error", err)
		h.Telegram.show(ctx, chatID, window.New, place{screen: screenHome}, c.TryLater())
	}
}

func (h *Home) homeView(ctx context.Context) window.View {
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
	return window.View{Text: c.Home(senderName(ctx), h.Telegram.BotName), Rows: rows}
}

// strangerView tells what the service is without revealing anybody's music.
func (h *Home) strangerView(ctx context.Context) window.View {
	c := texts(ctx)
	rows := [][]models.InlineKeyboardButton{{h.languageButton(c)}}
	stats, err := h.GetServiceStats.Execute(ctx)
	if err != nil {
		slog.Error("get_service_stats", "error", err)
		return window.View{Text: c.TryLater(), Rows: rows}
	}
	return window.View{Text: c.StrangerHome(stats.Users, stats.SharedTracks, h.AdminContact), Rows: rows}
}

// languageButton switches to the other language when there are two, and
// opens the list of languages otherwise.
func (h *Home) languageButton(c i18n.Catalog) models.InlineKeyboardButton {
	langs := h.Telegram.Texts.Languages()
	if len(langs) == 2 {
		other := langs[0]
		if other == c.Language() {
			other = langs[1]
		}
		return languageChoice(h.Telegram.Texts.For(other))
	}
	return models.InlineKeyboardButton{Text: c.LanguagesButton(), CallbackData: actionLanguage}
}

func languageChoice(c i18n.Catalog) models.InlineKeyboardButton {
	return models.InlineKeyboardButton{Text: c.LanguageButton(), CallbackData: actionLanguage + ":" + string(c.Language())}
}

// languagesView uses only language callbacks, so strangers can use it:
// back re-chooses the current language.
func (h *Home) languagesView(ctx context.Context) window.View {
	c := texts(ctx)
	rows := make([][]models.InlineKeyboardButton, 0, len(h.Telegram.Texts.Languages())+1)
	for _, lang := range h.Telegram.Texts.Languages() {
		rows = append(rows, []models.InlineKeyboardButton{languageChoice(h.Telegram.Texts.For(lang))})
	}
	back := models.InlineKeyboardButton{Text: c.Back(), CallbackData: actionLanguage + ":" + string(c.Language())}
	return window.View{Text: c.ChooseLanguage(), Rows: append(rows, []models.InlineKeyboardButton{back})}
}

// chooseLanguage lists the languages when lang is empty, else sets it.
func (h *Home) chooseLanguage(ctx context.Context, cb windowCallback, lang string) {
	h.Telegram.answerCallback(ctx, cb.query.ID, "")
	if lang == "" {
		h.Telegram.show(ctx, cb.chatID, cb.messageID, place{screen: screenLanguages}, "")
		return
	}
	c := h.Telegram.Texts.For(i18n.Language(lang))
	if err := h.Users.SetLanguage(ctx, cb.query.From.ID, string(c.Language())); err != nil {
		slog.Error("save_language", "error", err)
	}
	h.Telegram.show(withTexts(ctx, c), cb.chatID, cb.messageID, place{screen: screenHome}, "")
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
func (h *Home) inviteView(ctx context.Context) window.View {
	c := texts(ctx)
	home := place{screen: screenHome}
	code, err := h.CreateInvite.Execute(ctx)
	if errors.Is(err, access.ErrNotAdmin) {
		return h.homeView(ctx).WithNotice(c.NotAdmin())
	}
	if err != nil {
		slog.Error("create_invite", "error", err)
		return window.View{Text: c.InviteFailed(), Rows: [][]models.InlineKeyboardButton{backRow(ctx, home)}}
	}
	link := "https://t.me/" + h.Telegram.BotName + "?start=" + code
	return window.View{Text: c.Invite(link, h.CreateInvite.TTL), Rows: [][]models.InlineKeyboardButton{
		{goButton(c.AnotherInvite(), place{screen: screenInvite})},
		backRow(ctx, home),
	}}
}

func (h *Home) listenView(ctx context.Context) window.View {
	return window.View{Text: texts(ctx).Listen(), Rows: [][]models.InlineKeyboardButton{backRow(ctx, place{screen: screenHome})}}
}

func (h *Home) howToView(ctx context.Context) window.View {
	return window.View{Text: texts(ctx).HowTo(), Rows: [][]models.InlineKeyboardButton{backRow(ctx, place{screen: screenHome})}}
}
