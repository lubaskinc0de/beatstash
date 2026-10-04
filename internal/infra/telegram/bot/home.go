package bot

import (
	"context"
	"errors"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/beatstash/internal/application/common"
	"github.com/lubaskinc0de/beatstash/internal/application/common/listening"
	"github.com/lubaskinc0de/beatstash/internal/application/greet_stranger"
	"github.com/lubaskinc0de/beatstash/internal/application/invite_friend"
	"github.com/lubaskinc0de/beatstash/internal/application/join_by_invite"
	"github.com/lubaskinc0de/beatstash/internal/application/send_listen_link"
	"github.com/lubaskinc0de/beatstash/internal/application/view_home"
	"github.com/lubaskinc0de/beatstash/internal/domain/access"
	"github.com/lubaskinc0de/beatstash/internal/infra/telegram/i18n"
	"github.com/lubaskinc0de/beatstash/internal/infra/telegram/window"
)

// Home is the home screen, the invites and the language.
type Home struct {
	Telegram         *Telegram
	IDs              common.IDProvider
	Users            *Users
	GetHome          *view_home.GetHome
	CreateInvite     *invite_friend.CreateInvite
	AcceptInvite     *join_by_invite.AcceptInvite
	GetServiceStats  *greet_stranger.GetServiceStats
	CheckListenLinks *send_listen_link.CheckListenLinks
}

// handleStart with an invite code lets a stranger in and asks for a
// Navidrome login. The command goes once the window is up: only the window
// stays in the chat.
func (h *Home) handleStart(ctx context.Context, _ *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID
	defer h.Telegram.deleteMessage(ctx, chatID, update.Message.ID)
	_, code, _ := command(update)
	if _, err := h.IDs.CurrentUser(ctx); err != nil && code != "" {
		if !errors.Is(err, common.ErrNotAuthenticated) {
			slog.Error("authenticate", "error", err)
			return
		}
		h.join(ctx, chatID, code)
		return
	}
	at := place{screen: screenHome}
	current, err := h.Telegram.Windows.Get(ctx, chatID)
	if err != nil {
		slog.Error("get_start_window", "error", err)
		return
	}
	// A repeated start must not abandon account setup after the invite
	// has already been redeemed.
	if current != nil && (current.Screen == string(screenRegister) ||
		(current.Screen == string(screenLink) && current.Arg == argJoin)) {
		at = place{screen: screen(current.Screen), arg: current.Arg}
	}
	if code == startUnsendable {
		at = place{screen: screenUnsendable}
	}
	h.Telegram.show(ctx, chatID, window.New, at, "")
}

// startUnsendable comes from inline mode that left out Tracks it could not
// send.
const startUnsendable = "unsendable"

func (h *Home) unsendableView(ctx context.Context) window.View {
	c := texts(ctx)
	back := backRow(ctx, place{screen: screenHome})
	err := h.CheckListenLinks.Execute(ctx)
	switch {
	case err == nil:
		return window.View{Text: c.ListenLinksReady(), Rows: [][]models.InlineKeyboardButton{back}}
	case errors.Is(err, listening.ErrNoNavidromeAccount):
		return window.View{Text: c.UnsendableNoAccount(), Rows: [][]models.InlineKeyboardButton{
			{goButton(c.AccountsButton(), place{screen: screenNavidrome})}, back,
		}}
	case errors.Is(err, listening.ErrNoPublicAddress):
		return window.View{Text: c.UnsendableNoAddress(), Rows: [][]models.InlineKeyboardButton{back}}
	default:
		slog.Error("check_listen_links", "error", err)
		return window.View{Text: c.TryLater(), Rows: [][]models.InlineKeyboardButton{back}}
	}
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
	home, err := h.GetHome.Execute(ctx)
	if errors.Is(err, common.ErrNotAuthenticated) {
		return h.strangerView(ctx)
	}
	if err != nil {
		slog.Error("get_home", "error", err)
		home = &view_home.Home{}
	}

	rows := [][]models.InlineKeyboardButton{
		{goButton(c.MusicButton(), place{screen: screenShare})},
		{goButton(c.ImportButton(), place{screen: screenSources})},
		{goButton(c.HelpButton(), place{screen: screenHelp}), goButton(c.SettingsButton(), place{screen: screenSettings})},
	}
	if home.Admin {
		rows = append(rows, []models.InlineKeyboardButton{goButton(c.AdminButton(), place{screen: screenAdmin})})
	}
	return window.View{Text: c.Home(senderName(ctx), h.Telegram.BotName, home.Usage), Rows: rows}
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
	return window.View{Text: c.StrangerHome(stats.Users, stats.SharedTracks, h.Telegram.AdminContact), Rows: rows}
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

// chooseLanguage lists the languages when lang is empty, else sets it and
// returns to the settings, or Home for a stranger, who has none.
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
	back := place{screen: screenSettings}
	if _, err := h.IDs.CurrentUser(ctx); err != nil {
		back = place{screen: screenHome}
	}
	h.Telegram.show(withTexts(ctx, c), cb.chatID, cb.messageID, back, "")
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
	admin := place{screen: screenAdmin}
	code, err := h.CreateInvite.Execute(ctx)
	if errors.Is(err, access.ErrNotAdmin) {
		return h.homeView(ctx).WithNotice(c.NotAdmin())
	}
	if err != nil {
		slog.Error("create_invite", "error", err)
		return window.View{Text: c.InviteFailed(), Rows: [][]models.InlineKeyboardButton{backRow(ctx, admin)}}
	}
	link := "https://t.me/" + h.Telegram.BotName + "?start=" + code
	return window.View{Text: c.Invite(link, h.CreateInvite.TTL), Rows: [][]models.InlineKeyboardButton{
		{goButton(c.AnotherInvite(), place{screen: screenInvite})},
		backRow(ctx, admin),
	}}
}

func (h *Home) settingsView(ctx context.Context) window.View {
	c := texts(ctx)
	return window.View{Text: c.Settings(), Rows: [][]models.InlineKeyboardButton{
		{goButton(c.AccountsButton(), place{screen: screenNavidrome})},
		{h.languageButton(c)},
		backRow(ctx, place{screen: screenHome}),
	}}
}

func (h *Home) helpView(ctx context.Context) window.View {
	c := texts(ctx)
	return window.View{Text: c.Help(), Rows: [][]models.InlineKeyboardButton{
		{goButton(c.HowToButton(), place{screen: screenHowTo}), goButton(c.ListenButton(), place{screen: screenListen})},
		{goButton(c.InlineHelpButton(), place{screen: screenInlineHelp})},
		{goButton(c.SpaceHelpButton(), place{screen: screenSpaceHelp})},
		backRow(ctx, place{screen: screenHome}),
	}}
}

func helpSection(ctx context.Context, text string) window.View {
	return window.View{Text: text, Rows: [][]models.InlineKeyboardButton{backRow(ctx, place{screen: screenHelp})}}
}

func (h *Home) howToView(ctx context.Context) window.View {
	return helpSection(ctx, texts(ctx).HowTo())
}

func (h *Home) listenView(ctx context.Context) window.View {
	return helpSection(ctx, texts(ctx).Listen())
}

func (h *Home) inlineHelpView(ctx context.Context) window.View {
	return helpSection(ctx, texts(ctx).InlineHelp(h.Telegram.BotName))
}

func (h *Home) spaceHelpView(ctx context.Context) window.View {
	return helpSection(ctx, texts(ctx).SpaceHelp(h.Telegram.AdminContact))
}
