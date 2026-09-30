package i18n

import (
	"strings"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/manage_quotas"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/oversee_service"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

func (c Catalog) AdminButton() string  { return c.t("button.admin", nil) }
func (c Catalog) UsersButton() string  { return c.t("admin.users_button", nil) }
func (c Catalog) QuotasButton() string { return c.t("admin.quotas_button", nil) }
func (c Catalog) PrevPage() string     { return c.t("admin.prev", nil) }
func (c Catalog) NextPage() string     { return c.t("admin.next", nil) }
func (c Catalog) AdminOnly() string    { return c.t("admin.only", nil) }
func (c Catalog) UsersTitle() string   { return c.t("admin.users_title", nil) }

func (c Catalog) EditDefaultQuota() string   { return c.t("quota.edit_default", nil) }
func (c Catalog) EditSharedQuota() string    { return c.t("quota.edit_shared", nil) }
func (c Catalog) EditUserQuota() string      { return c.t("quota.edit_user", nil) }
func (c Catalog) UnlimitedButton() string    { return c.t("quota.unlimited_button", nil) }
func (c Catalog) FromConfigButton() string   { return c.t("quota.from_config_button", nil) }
func (c Catalog) DefaultQuotaButton() string { return c.t("quota.default_button", nil) }
func (c Catalog) QuotaUnreadable() string    { return c.t("quota.unreadable", nil) }
func (c Catalog) OvercommitWarning() string  { return c.t("quota.overcommitted", nil) }

func (c Catalog) PresetButton(q library.Quota) string { return c.quota(q) }

func (c Catalog) Overview(o *oversee_service.Overview) string {
	return c.t("admin.title", nil) + "\n\n" + c.ServiceStats(o.Stats) + "\n\n" + c.Capacity(o.Capacity)
}

func (c Catalog) ServerQuotas(q *manage_quotas.ServerQuotas) string {
	return c.t("quota.title", nil) + "\n\n" +
		c.serverQuota("quota.default", q.Default) + "\n" +
		c.serverQuota("quota.shared", q.Shared) + "\n\n" +
		c.Capacity(q.Capacity)
}

func (c Catalog) DefaultQuotaEdit(q manage_quotas.ServerQuota) string {
	return c.serverQuota("quota.default", q) + "\n\n" + c.t("quota.prompt", nil)
}

func (c Catalog) SharedQuotaEdit(q manage_quotas.ServerQuota) string {
	return c.serverQuota("quota.shared", q) + "\n\n" + c.t("quota.prompt", nil)
}

func (c Catalog) UserQuotaEdit(card *oversee_service.UserCard) string {
	return "👤 <b>" + esc(c.userName(card.User)) + "</b>\n" +
		c.CardUsage(card.Tracks, card.Usage, card.OwnQuota) + "\n\n" + c.t("quota.prompt", nil)
}

// serverQuota marks a Quota the Admin has not set.
func (c Catalog) serverQuota(id string, q manage_quotas.ServerQuota) string {
	text := c.t(id, args{"Quota": c.quota(q.Quota)})
	if q.FromConfig {
		text += " " + c.t("quota.from_config", nil)
	}
	return text
}

// Capacity warns when the Quotas promise more than the disk holds.
func (c Catalog) Capacity(capacity library.Capacity) string {
	text := c.Promised(capacity.Promised) + " · " + c.CapacityUsed(capacity.Used) + " · " +
		c.t("quota.free", args{"Free": c.size(capacity.Free)})
	if capacity.Overcommitted() {
		text += "\n" + c.OvercommitWarning()
	}
	return text
}

// CapacityUsed is what the bot's Libraries weigh together.
func (c Catalog) CapacityUsed(used int64) string {
	return c.t("quota.used", args{"Used": c.size(used)})
}

func (c Catalog) Promised(q library.Quota) string {
	promised := "∞"
	if q.Limited() {
		promised = c.size(int64(q))
	}
	return c.t("quota.promised", args{"Promised": promised})
}

func (c Catalog) ServiceStats(s oversee_service.ServiceStats) string {
	return strings.Join([]string{
		c.t("admin.users", args{"Users": s.Users, "Active": s.ActiveUsers}),
		c.t("admin.strangers", args{"Strangers": s.Strangers}),
		c.t("admin.tracks", args{"Tracks": s.Tracks, "Shared": s.SharedTracks, "Takes": s.Takes}),
		c.t("admin.jobs", args{"Pending": s.PendingJobs, "Failed": s.FailedJobs}),
	}, "\n")
}

// UserButton is plain text: the name, the username and Last Seen.
func (c Catalog) UserButton(user *access.User, now time.Time) string {
	who := c.userName(user)
	if name := fullName(user); name != "" && user.Username != "" {
		who = name + " @" + user.Username
	}
	return who + " · " + c.lastSeen(user, now)
}

// UserCard links to the profile in the Channel when there is a link.
func (c Catalog) UserCard(card *oversee_service.UserCard, link string, now time.Time) string {
	user := card.User
	lines := []string{"👤 <b>" + esc(c.userName(user)) + "</b>"}
	var who []string
	if user.Username != "" {
		who = append(who, "@"+esc(user.Username))
	}
	if link != "" {
		who = append(who, `<a href="`+esc(link)+`">`+c.t("admin.profile", nil)+"</a>")
	}
	if len(who) > 0 {
		lines = append(lines, strings.Join(who, " · "))
	}
	lines = append(lines,
		c.t("admin.card_dates", args{"Since": user.CreatedAt.Format(time.DateOnly), "Seen": c.lastSeen(user, now)}),
		c.CardUsage(card.Tracks, card.Usage, card.OwnQuota),
	)
	return strings.Join(lines, "\n")
}

// CardUsage tells whether the Quota is the User's own or the Default Quota.
func (c Catalog) CardUsage(tracks int64, usage library.Usage, own bool) string {
	source := c.t("admin.default_quota", nil)
	if own {
		source = c.t("admin.own_quota", nil)
	}
	return c.t("admin.card_usage", with(with(c.usageArgs(usage), "Tracks", tracks), "Source", source))
}

func (c Catalog) lastSeen(user *access.User, now time.Time) string {
	if user.LastSeenAt == nil {
		return c.t("admin.never_seen", nil)
	}
	return c.ago(*user.LastSeenAt, now)
}

// userName falls back to the username, then to the id.
func (c Catalog) userName(user *access.User) string {
	if name := fullName(user); name != "" {
		return name
	}
	if user.Username != "" {
		return "@" + user.Username
	}
	return c.t("admin.user_id", args{"ID": user.ID})
}

func fullName(user *access.User) string {
	return strings.TrimSpace(user.FirstName + " " + user.LastName)
}
