package bot

import (
	"context"

	"github.com/go-telegram/bot"

	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/i18n"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/window"
)

// Telegram is what every feature draws and writes with.
type Telegram struct {
	Bot     *bot.Bot
	BotName string
	Windows *window.Windows
	Texts   *i18n.Bundle
	// AdminContact is who users ask for access and space; empty hides it.
	AdminContact string

	// screens draws any screen: a feature's action may lead to another's.
	screens *Handler
}

// Handler routes the updates, screens, texts and buttons to the features.
type Handler struct {
	Telegram  *Telegram
	Home      *Home
	Feed      *Feed
	Imports   *Imports
	Navidrome *Navidrome
	Inline    *Inline
	Uploads   *Uploads
	Admin     *Admin
	Quotas    *Quotas
	Share     *ShareScreen
}

// Register adds the routes; they never match the same update.
func (h *Handler) Register() {
	h.Telegram.screens = h
	b := h.Telegram.Bot
	b.RegisterHandlerMatchFunc(hasAudio, h.Uploads.handleAudio)
	b.RegisterHandlerMatchFunc(isCommand("start"), h.Home.handleStart)
	b.RegisterHandlerMatchFunc(isText, h.handleText)
	b.RegisterHandlerMatchFunc(isInlineQuery, h.Inline.handleInlineQuery)
	b.RegisterHandlerMatchFunc(isCallbackQuery, h.handleCallbackQuery)
	b.RegisterHandlerMatchFunc(isChosenInlineResult, h.Inline.handleChosenInlineResult)
}

func (h *Handler) view(ctx context.Context, s screen, arg string) window.View {
	switch s {
	case screenFeed:
		return h.Feed.feedView(ctx)
	case screenSharedTrack:
		return h.Feed.sharedTrackView(ctx, arg)
	case screenTop:
		return h.Feed.topView(ctx)
	case screenSources:
		return h.Imports.sourcesView(ctx)
	case screenProvider:
		return h.Imports.providerView(ctx, arg)
	case screenConnect:
		return h.Imports.connectView(ctx, arg)
	case screenPlan:
		return h.Imports.planView(ctx, arg)
	case screenImports:
		return h.Imports.importsView(ctx)
	case screenNavidrome:
		return h.Navidrome.navidromeView(ctx)
	case screenLink:
		return h.Navidrome.linkView(ctx, arg)
	case screenRegister:
		return h.Navidrome.registerView(ctx)
	case screenInvite:
		return h.Home.inviteView(ctx)
	case screenHelp:
		return h.Home.helpView(ctx)
	case screenHowTo:
		return h.Home.howToView(ctx)
	case screenListen:
		return h.Home.listenView(ctx)
	case screenInlineHelp:
		return h.Home.inlineHelpView(ctx)
	case screenSpaceHelp:
		return h.Home.spaceHelpView(ctx)
	case screenSettings:
		return h.Home.settingsView(ctx)
	case screenLanguages:
		return h.Home.languagesView(ctx)
	case screenAdmin:
		return h.Admin.adminView(ctx)
	case screenUsers:
		return h.Admin.usersView(ctx, arg)
	case screenUser:
		return h.Admin.userView(ctx, arg)
	case screenQuotas:
		return h.Quotas.quotasView(ctx)
	case screenQuota:
		return h.Quotas.quotaView(ctx, arg)
	case screenUserQuota:
		return h.Quotas.userQuotaView(ctx, arg)
	case screenShare:
		return h.Share.shareView(ctx, arg)
	case screenShareTrack:
		return h.Share.trackCardView(ctx, arg)
	case screenShareAlbum:
		return h.Share.albumCardView(ctx, arg)
	case screenUnsendable:
		return h.Home.unsendableView(ctx)
	default:
		return h.Home.homeView(ctx)
	}
}

// onText returns nil for a screen that awaits no text.
func (h *Handler) onText(s screen) func(context.Context, windowInput) {
	switch s {
	case screenConnect:
		return h.Imports.connectProvider
	case screenLink:
		return h.Navidrome.linkNavidrome
	case screenRegister:
		return h.Navidrome.registerNavidrome
	case screenQuota, screenUserQuota:
		return h.Quotas.typeQuota(s)
	case screenShare:
		return h.Share.typeQuery
	default:
		return nil
	}
}
