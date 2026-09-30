package bot

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/oversee_service"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/window"
)

// Admin is the Admin's screens: the overview, the users and their cards.
type Admin struct {
	Telegram    *Telegram
	GetOverview *oversee_service.GetOverview
	ListUsers   *oversee_service.ListUsers
	GetUserCard *oversee_service.GetUserCard
	// Clock tells how long ago users were seen.
	Clock func() time.Time
}

func (a *Admin) adminView(ctx context.Context) window.View {
	c := texts(ctx)
	overview, err := a.GetOverview.Execute(ctx)
	if err != nil {
		return adminFailed(ctx, a.Telegram, "get_overview", err, place{screen: screenHome})
	}
	return window.View{Text: c.Overview(overview), Rows: [][]models.InlineKeyboardButton{
		{goButton(c.UsersButton(), place{screen: screenUsers}), goButton(c.QuotasButton(), place{screen: screenQuotas})},
		backRow(ctx, place{screen: screenHome}),
	}}
}

// usersView takes the page, counted from zero.
func (a *Admin) usersView(ctx context.Context, arg string) window.View {
	c := texts(ctx)
	page, _ := strconv.Atoi(arg)
	users, err := a.ListUsers.Execute(ctx, page)
	if err != nil {
		return adminFailed(ctx, a.Telegram, "list_users", err, place{screen: screenAdmin})
	}
	now := a.Clock()
	rows := make([][]models.InlineKeyboardButton, 0, len(users.Users)+2)
	for n := range users.Users {
		user := &users.Users[n]
		rows = append(rows, []models.InlineKeyboardButton{
			goButton(c.UserButton(user, now), place{screen: screenUser, arg: userArg(user.ID, users.Page)}),
		})
	}
	var pages []models.InlineKeyboardButton
	if users.Page > 0 {
		pages = append(pages, goButton(c.PrevPage(), place{screen: screenUsers, arg: strconv.Itoa(users.Page - 1)}))
	}
	if users.More {
		pages = append(pages, goButton(c.NextPage(), place{screen: screenUsers, arg: strconv.Itoa(users.Page + 1)}))
	}
	if len(pages) > 0 {
		rows = append(rows, pages)
	}
	return window.View{Text: c.UsersTitle(), Rows: append(rows, backRow(ctx, place{screen: screenAdmin}))}
}

// userView takes "<user id>:<page>": back leads to the page of the list.
func (a *Admin) userView(ctx context.Context, arg string) window.View {
	c := texts(ctx)
	userID, page := parseUserArg(arg)
	list := place{screen: screenUsers, arg: strconv.Itoa(page)}
	card, err := a.GetUserCard.Execute(ctx, userID)
	if err != nil {
		return adminFailed(ctx, a.Telegram, "get_user_card", err, list)
	}
	return window.View{Text: c.UserCard(card, profileLink(card.User), a.Clock()), Rows: [][]models.InlineKeyboardButton{
		{goButton(c.EditUserQuota(), place{screen: screenUserQuota, arg: arg})},
		backRow(ctx, list),
	}}
}

// adminFailed leads a non-Admin Home.
func adminFailed(ctx context.Context, t *Telegram, op string, err error, back place) window.View {
	c := texts(ctx)
	if errors.Is(err, access.ErrNotAdmin) {
		return t.screens.view(ctx, screenHome, "").WithNotice(c.AdminOnly())
	}
	slog.Error(op, "error", err)
	return window.View{Text: c.TryLater(), Rows: [][]models.InlineKeyboardButton{backRow(ctx, back)}}
}

func userArg(userID uint, page int) string {
	return strconv.FormatUint(uint64(userID), 10) + ":" + strconv.Itoa(page)
}

func parseUserArg(arg string) (userID uint, page int) {
	id, rest, _ := strings.Cut(arg, ":")
	n, _ := strconv.ParseUint(id, 10, 64)
	page, _ = strconv.Atoi(rest)
	return uint(n), page
}

// profileLink opens the User's Telegram profile; empty without a Telegram
// Identity.
func profileLink(user *access.User) string {
	for _, identity := range user.Identities {
		if identity.Channel == Channel {
			return "tg://user?id=" + identity.ExternalID
		}
	}
	return ""
}
