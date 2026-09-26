package bot

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/navidrome"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/connect_navidrome"
)

func (h *Handler) navidromeView(ctx context.Context) view {
	c := texts(ctx)
	back := backRow(ctx, place{screen: screenHome})
	account, err := h.GetNavidromeAccount.Execute(ctx)
	if err != nil {
		slog.Error("get_navidrome_account", "error", err)
		return view{text: c.TryLater(), rows: [][]models.InlineKeyboardButton{back}}
	}
	link := place{screen: screenLink}
	if account == nil {
		return view{text: c.NavidromeNotLinked(), rows: [][]models.InlineKeyboardButton{{goButton(c.Link(), link)}, back}}
	}
	return view{text: c.NavidromeLinked(account.Login), rows: [][]models.InlineKeyboardButton{{goButton(c.LinkAnother(), link)}, back}}
}

// linkView in the onboarding cancels back to choosing a login.
func (h *Handler) linkView(ctx context.Context, arg string) view {
	cancel := place{screen: screenNavidrome}
	if arg == argJoin {
		cancel = place{screen: screenRegister}
	}
	return view{text: texts(ctx).LinkPrompt(), rows: [][]models.InlineKeyboardButton{{goButton(texts(ctx).Cancel(), cancel)}}}
}

func (h *Handler) registerView(ctx context.Context) view {
	c := texts(ctx)
	return view{text: c.ChooseLogin(), rows: [][]models.InlineKeyboardButton{
		{goButton(c.HaveAccount(), place{screen: screenLink, arg: argJoin})},
	}}
}

// linkNavidrome deletes the message at once: it holds a password.
func (h *Handler) linkNavidrome(ctx context.Context, b *bot.Bot, in windowInput) {
	c := texts(ctx)
	deleteMessage(ctx, b, in.chatID, in.messageID)
	here := place{screen: screenLink, arg: in.arg}
	done := place{screen: screenNavidrome}
	if in.arg == argJoin {
		done = place{screen: screenHome}
	}

	login, password, _ := strings.Cut(in.text, " ")
	password = strings.TrimSpace(password)
	if login == "" || password == "" {
		h.showInWindow(ctx, b, in.chatID, here, c.LinkMalformed())
		return
	}

	err := h.LinkNavidromeAccount.Execute(ctx, navidrome.Credentials{Login: login, Password: password})
	switch {
	case err == nil:
		h.showInWindow(ctx, b, in.chatID, done, c.Linked(login))
	case errors.Is(err, navidrome.ErrAdminAccount):
		h.showInWindow(ctx, b, in.chatID, done, c.LinkedAdmin(login))
	case errors.Is(err, connect_navidrome.ErrNavidromeAccountTaken):
		h.showInWindow(ctx, b, in.chatID, here, c.LinkTaken())
	case errors.Is(err, navidrome.ErrInvalidCredentials):
		h.showInWindow(ctx, b, in.chatID, here, c.LinkWrongPassword())
	default:
		slog.Error("link_navidrome_account", "error", err)
		h.showInWindow(ctx, b, in.chatID, here, c.LinkFailed())
	}
}

// registerNavidrome sends the password separately, so the user can delete it.
func (h *Handler) registerNavidrome(ctx context.Context, b *bot.Bot, in windowInput) {
	c := texts(ctx)
	here := place{screen: screenRegister}
	creds, err := h.RegisterAccount.Execute(ctx, in.text)
	switch {
	case err == nil:
		h.showInWindow(ctx, b, in.chatID, place{screen: screenHome}, "")
		sendText(ctx, b, in.chatID, c.Registered(creds.Login, creds.Password))
	case errors.Is(err, connect_navidrome.ErrHasNavidromeAccount):
		h.showInWindow(ctx, b, in.chatID, place{screen: screenHome}, "")
	case errors.Is(err, connect_navidrome.ErrNavidromeLoginInvalid):
		h.showInWindow(ctx, b, in.chatID, here, c.LoginInvalid())
	case errors.Is(err, navidrome.ErrLoginTaken):
		h.showInWindow(ctx, b, in.chatID, here, c.LoginTaken(in.text))
	default:
		slog.Error("register_navidrome_account", "error", err)
		h.showInWindow(ctx, b, in.chatID, here, c.RegisterFailed())
	}
}
