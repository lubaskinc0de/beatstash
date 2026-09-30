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
	Sharing   *Sharing
	Inline    *Inline
	Uploads   *Uploads
}

// Register adds the routes; they never match the same update.
func (h *Handler) Register() {
	h.Telegram.screens = h
	b := h.Telegram.Bot
	b.RegisterHandlerMatchFunc(hasAudio, h.Uploads.handleAudio)
	b.RegisterHandlerMatchFunc(isCommand("start"), h.Home.handleStart)
	b.RegisterHandlerMatchFunc(isCommand("share"), h.Sharing.handleShare)
	b.RegisterHandlerMatchFunc(isText, h.handleText)
	b.RegisterHandlerMatchFunc(isInlineQuery, h.Inline.handleInlineQuery)
	b.RegisterHandlerMatchFunc(isCallbackQuery, h.handleCallbackQuery)
	b.RegisterHandlerMatchFunc(isChosenInlineResult, h.Inline.handleChosenInlineResult)
}

func (h *Handler) view(ctx context.Context, s screen, arg string) window.View {
	switch s {
	case screenFeed:
		return h.Feed.feedView(ctx)
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
	case screenHowTo:
		return h.Home.howToView(ctx)
	case screenListen:
		return h.Home.listenView(ctx)
	case screenLanguages:
		return h.Home.languagesView(ctx)
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
	default:
		return nil
	}
}
