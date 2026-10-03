package bot

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"

	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/beatstash/internal/application/manage_quotas"
	"github.com/lubaskinc0de/beatstash/internal/application/oversee_service"
	"github.com/lubaskinc0de/beatstash/internal/domain/access"
	"github.com/lubaskinc0de/beatstash/internal/domain/library"
	"github.com/lubaskinc0de/beatstash/internal/infra/telegram/window"
)

// Quotas is the Admin's screens of the Quotas: the server's and a user's.
type Quotas struct {
	Telegram        *Telegram
	GetServerQuotas *manage_quotas.GetServerQuotas
	SetDefaultQuota *manage_quotas.SetDefaultQuota
	SetSharedQuota  *manage_quotas.SetSharedQuota
	SetUserQuota    *manage_quotas.SetUserQuota
	GetUserCard     *oversee_service.GetUserCard
}

// A Quota's target is quotaDefault, quotaShared or a user's userArg.
const (
	quotaDefault = "default"
	quotaShared  = "shared"
)

// In a button a Quota is its bytes, "0" for unlimited, or quotaReset to
// follow the config or the Default Quota again.
const quotaReset = "-"

var quotaPresets = []library.Quota{1 << 30, 5 << 30, 10 << 30, 50 << 30}

func (q *Quotas) quotasView(ctx context.Context) window.View {
	c := texts(ctx)
	quotas, err := q.GetServerQuotas.Execute(ctx)
	if err != nil {
		return adminFailed(ctx, q.Telegram, "get_server_quotas", err, place{screen: screenAdmin})
	}
	return window.View{Text: c.ServerQuotas(quotas), Rows: [][]models.InlineKeyboardButton{
		{
			goButton(c.EditDefaultQuota(), place{screen: screenQuota, arg: quotaDefault}),
			goButton(c.EditSharedQuota(), place{screen: screenQuota, arg: quotaShared}),
		},
		backRow(ctx, place{screen: screenAdmin}),
	}}
}

// quotaView edits the Default Quota or the Shared Library's.
func (q *Quotas) quotaView(ctx context.Context, target string) window.View {
	c := texts(ctx)
	back := place{screen: screenQuotas}
	quotas, err := q.GetServerQuotas.Execute(ctx)
	if err != nil {
		return adminFailed(ctx, q.Telegram, "get_server_quotas", err, back)
	}
	text := c.DefaultQuotaEdit(quotas.Default)
	if target == quotaShared {
		text = c.SharedQuotaEdit(quotas.Shared)
	}
	return window.View{Text: text, Rows: quotaRows(ctx, target, c.FromConfigButton(), back)}
}

// userQuotaView takes the userArg of the card it came from.
func (q *Quotas) userQuotaView(ctx context.Context, arg string) window.View {
	c := texts(ctx)
	back := place{screen: screenUser, arg: arg}
	userID, _ := parseUserArg(arg)
	card, err := q.GetUserCard.Execute(ctx, userID)
	if err != nil {
		return adminFailed(ctx, q.Telegram, "get_user_card", err, back)
	}
	return window.View{Text: c.UserQuotaEdit(card), Rows: quotaRows(ctx, arg, c.DefaultQuotaButton(), back)}
}

func quotaRows(ctx context.Context, target, reset string, back place) [][]models.InlineKeyboardButton {
	c := texts(ctx)
	presets := make([]models.InlineKeyboardButton, 0, len(quotaPresets))
	for _, preset := range quotaPresets {
		presets = append(presets, quotaButton(c.PresetButton(preset), strconv.FormatInt(int64(preset), 10), target))
	}
	return [][]models.InlineKeyboardButton{
		presets,
		{quotaButton(c.UnlimitedButton(), "0", target), quotaButton(reset, quotaReset, target)},
		backRow(ctx, back),
	}
}

func quotaButton(text, value, target string) models.InlineKeyboardButton {
	return models.InlineKeyboardButton{Text: text, CallbackData: actionQuota + ":" + value + ":" + target}
}

func (q *Quotas) chooseQuota(ctx context.Context, cb windowCallback, arg string) {
	q.Telegram.answerCallback(ctx, cb.query.ID, "")
	value, target, _ := strings.Cut(arg, ":")
	var quota *library.Quota
	if value != quotaReset {
		bytes, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return
		}
		chosen, err := library.NewQuota(bytes)
		if err != nil {
			return
		}
		quota = &chosen
	}
	q.set(ctx, cb.chatID, cb.messageID, target, quota)
}

// typeQuota reads a size the Admin sent; an unreadable one leaves the
// screen waiting for another.
func (q *Quotas) typeQuota(here screen) func(context.Context, windowInput) {
	return func(ctx context.Context, in windowInput) {
		bytes, ok := texts(ctx).ParseSize(in.text)
		quota, err := library.NewQuota(bytes)
		if !ok || err != nil {
			q.Telegram.show(ctx, in.chatID, window.Current, place{screen: here, arg: in.arg}, texts(ctx).QuotaUnreadable())
			return
		}
		q.set(ctx, in.chatID, window.Current, in.arg, &quota)
	}
}

// set shows the screen the Quota belongs to.
func (q *Quotas) set(ctx context.Context, chatID int64, on int, target string, quota *library.Quota) {
	c := texts(ctx)
	var err error
	done := place{screen: screenQuotas}
	switch target {
	case quotaDefault:
		err = q.SetDefaultQuota.Execute(ctx, quota)
	case quotaShared:
		err = q.SetSharedQuota.Execute(ctx, quota)
	default:
		userID, _ := parseUserArg(target)
		err = q.SetUserQuota.Execute(ctx, userID, quota)
		done = place{screen: screenUser, arg: target}
	}
	switch {
	case err == nil:
		q.Telegram.show(ctx, chatID, on, done, "")
	case errors.Is(err, access.ErrNotAdmin):
		q.Telegram.show(ctx, chatID, on, place{screen: screenHome}, c.AdminOnly())
	default:
		slog.Error("set_quota", "target", target, "error", err)
		q.Telegram.show(ctx, chatID, on, done, c.TryLater())
	}
}
