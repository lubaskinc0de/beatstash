package bot

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/beatstash/internal/application/common/navidrome"
	"github.com/lubaskinc0de/beatstash/internal/application/connect_navidrome"
	"github.com/lubaskinc0de/beatstash/internal/infra/telegram/window"
)

// Navidrome links and registers the Navidrome Account.
type Navidrome struct {
	Telegram             *Telegram
	GetNavidromeAccount  *connect_navidrome.GetNavidromeAccount
	LinkNavidromeAccount *connect_navidrome.LinkNavidromeAccount
	RegisterAccount      *connect_navidrome.RegisterNavidromeAccount
}

func (n *Navidrome) navidromeView(ctx context.Context) window.View {
	c := texts(ctx)
	back := backRow(ctx, place{screen: screenSettings})
	account, err := n.GetNavidromeAccount.Execute(ctx)
	if err != nil {
		slog.Error("get_navidrome_account", "error", err)
		return window.View{Text: c.TryLater(), Rows: [][]models.InlineKeyboardButton{back}}
	}
	link := place{screen: screenLink}
	if account == nil {
		return window.View{Text: c.NavidromeNotLinked(), Rows: [][]models.InlineKeyboardButton{{styled(goButton(c.Link(), link), stylePrimary)}, back}}
	}
	return window.View{Text: c.NavidromeLinked(account.Login), Rows: [][]models.InlineKeyboardButton{{goButton(c.LinkAnother(), link)}, back}}
}

// linkView in the onboarding cancels back to choosing a login.
func (n *Navidrome) linkView(ctx context.Context, arg string) window.View {
	cancel := place{screen: screenNavidrome}
	if arg == argJoin {
		cancel = place{screen: screenRegister}
	}
	return window.View{Text: texts(ctx).LinkPrompt(), Rows: [][]models.InlineKeyboardButton{{styled(goButton(texts(ctx).Cancel(), cancel), styleDanger)}}}
}

func (n *Navidrome) registerView(ctx context.Context) window.View {
	c := texts(ctx)
	return window.View{Text: c.ChooseLogin(), Rows: [][]models.InlineKeyboardButton{
		{goButton(c.HaveAccount(), place{screen: screenLink, arg: argJoin})},
	}}
}

func (n *Navidrome) linkNavidrome(ctx context.Context, in windowInput) {
	c := texts(ctx)
	here := place{screen: screenLink, arg: in.arg}
	done := place{screen: screenNavidrome}
	if in.arg == argJoin {
		done = place{screen: screenHome}
	}

	login, password, _ := strings.Cut(in.text, " ")
	password = strings.TrimSpace(password)
	if login == "" || password == "" {
		n.Telegram.show(ctx, in.chatID, window.Current, here, c.LinkMalformed())
		return
	}

	songs, err := n.LinkNavidromeAccount.Execute(ctx, navidrome.Credentials{Login: login, Password: password})
	switch {
	case err == nil:
		n.Telegram.show(ctx, in.chatID, window.Current, done, c.Linked(login, songs))
	case errors.Is(err, navidrome.ErrAdminAccount):
		n.Telegram.show(ctx, in.chatID, window.Current, done, c.LinkedAdmin(login, songs))
	case errors.Is(err, connect_navidrome.ErrNavidromeAccountTaken):
		n.Telegram.show(ctx, in.chatID, window.Current, here, c.LinkTaken())
	case errors.Is(err, navidrome.ErrInvalidCredentials):
		n.Telegram.show(ctx, in.chatID, window.Current, here, c.LinkWrongPassword())
	default:
		slog.Error("link_navidrome_account", "error", err)
		n.Telegram.show(ctx, in.chatID, window.Current, here, c.LinkFailed())
	}
}

// registerNavidrome sends the password separately, so the user can delete it.
func (n *Navidrome) registerNavidrome(ctx context.Context, in windowInput) {
	c := texts(ctx)
	here := place{screen: screenRegister}
	creds, err := n.RegisterAccount.Execute(ctx, in.text)
	switch {
	case err == nil:
		n.Telegram.show(ctx, in.chatID, window.Current, place{screen: screenHome}, "")
		logUnsent(n.Telegram.sendText(ctx, in.chatID, c.Registered(creds.Login, creds.Password)))
	case errors.Is(err, connect_navidrome.ErrHasNavidromeAccount):
		n.Telegram.show(ctx, in.chatID, window.Current, place{screen: screenHome}, "")
	case errors.Is(err, connect_navidrome.ErrNavidromeLoginInvalid):
		n.Telegram.show(ctx, in.chatID, window.Current, here, c.LoginInvalid())
	case errors.Is(err, navidrome.ErrLoginTaken):
		n.Telegram.show(ctx, in.chatID, window.Current, here, c.LoginTaken(in.text))
	default:
		slog.Error("register_navidrome_account", "error", err)
		n.Telegram.show(ctx, in.chatID, window.Current, here, c.RegisterFailed())
	}
}
