package i18n

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"

	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"

	"github.com/lubaskinc0de/beatstash/internal/application/browse_shared"
	"github.com/lubaskinc0de/beatstash/internal/application/common/repositories"
	"github.com/lubaskinc0de/beatstash/internal/application/import_collection"
	"github.com/lubaskinc0de/beatstash/internal/application/share_tracks"
	"github.com/lubaskinc0de/beatstash/internal/application/show_playing"
	"github.com/lubaskinc0de/beatstash/internal/application/view_top"
	"github.com/lubaskinc0de/beatstash/internal/domain/access"
	"github.com/lubaskinc0de/beatstash/internal/domain/ingest"
	"github.com/lubaskinc0de/beatstash/internal/domain/library"
	"github.com/lubaskinc0de/beatstash/internal/domain/provider"
)

type Article struct {
	Title       string
	Description string
	Message     string
}

type Hint struct {
	Command string
	Article
}

// Catalog keeps text keys and their arguments in one place. Texts are HTML
// except buttons, callback answers and bot descriptions.
type Catalog struct {
	lang  Language
	loc   *goi18n.Localizer
	brand Brand
}

type args = map[string]any

func (c Catalog) Language() Language { return c.lang }

func (c Catalog) LanguageName() string {
	if name, ok := c.lookup("language.name", nil, nil); ok {
		return name
	}
	return string(c.lang)
}

func (c Catalog) LanguageButton() string { return "🌐 " + c.LanguageName() }

func (c Catalog) BotDescription() string      { return c.t("bot.about", nil) }
func (c Catalog) BotShortDescription() string { return c.t("bot.short_description", nil) }
func (c Catalog) StartCommand() string        { return c.t("bot.start_command", nil) }

func (c Catalog) MusicButton() string     { return c.t("button.music", nil) }
func (c Catalog) SharedTab() string       { return c.t("music.shared_tab", nil) }
func (c Catalog) MineTab() string         { return c.t("music.mine_tab", nil) }
func (c Catalog) TopButton() string       { return c.t("button.top", nil) }
func (c Catalog) ImportButton() string    { return c.t("button.import", nil) }
func (c Catalog) HelpButton() string      { return c.t("button.help", nil) }
func (c Catalog) SettingsButton() string  { return c.t("button.settings", nil) }
func (c Catalog) AccountsButton() string  { return c.t("button.accounts", nil) }
func (c Catalog) ListenButton() string    { return c.t("button.listen", nil) }
func (c Catalog) InviteButton() string    { return c.t("button.invite", nil) }
func (c Catalog) HowToButton() string     { return c.t("button.how_to", nil) }
func (c Catalog) LanguagesButton() string { return c.t("button.languages", nil) }
func (c Catalog) Back() string            { return c.t("button.back", nil) }
func (c Catalog) Cancel() string          { return c.t("button.cancel", nil) }
func (c Catalog) TryLater() string        { return c.t("try_later", nil) }
func (c Catalog) ChooseLanguage() string  { return c.t("languages", nil) }

// Home tells the Usage only under a limited Quota.
func (c Catalog) Home(name, bot string, usage library.Usage) string {
	text := c.t("home.greeting", nil)
	if name != "" {
		text = c.t("home.greeting_named", args{"Name": esc(name)})
	}
	text += "\n\n" + c.t("home.text", args{"Bot": esc(bot)})
	if usage.Quota.Limited() {
		text += "\n\n" + c.t("home.usage", c.usageArgs(usage))
	}
	return text + "\n\n" + c.t("home.more", nil) + "\n" + c.footer()
}

func (c Catalog) StrangerHome(users, sharedTracks int64, contact string) string {
	text := c.t("stranger.text", args{"Users": users, "SharedTracks": sharedTracks})
	if contact != "" {
		text += "\n" + c.t("stranger.contact", args{"Contact": esc(contact)})
	}
	return text + "\n\n" + c.footer()
}

func (c Catalog) footer() string {
	return footer(c.t("footer.source", nil), c.t("footer.author", nil))
}

func (c Catalog) Settings() string { return c.t("settings.title", nil) }

func (c Catalog) Help() string             { return c.t("help.text", nil) }
func (c Catalog) InlineHelpButton() string { return c.t("help.inline_button", nil) }
func (c Catalog) SpaceHelpButton() string  { return c.t("help.space_button", nil) }
func (c Catalog) HowTo() string            { return c.t("how_to", nil) }
func (c Catalog) Listen() string           { return c.t("listen", nil) }

func (c Catalog) InlineHelp(bot string) string {
	var b strings.Builder
	b.WriteString(c.t("help.inline", nil))
	for _, command := range inlineCommands {
		fmt.Fprintf(&b, "\n%s — %s", inlineUsage(bot, command.name), c.t("help."+command.name, nil))
	}
	b.WriteString("\n\n" + c.t("help.search", args{"Usage": inlineUsage(bot, "queen")}))
	return b.String()
}

func (c Catalog) SpaceHelp(contact string) string {
	return c.withContact(c.t("help.space", nil), esc(contact))
}

func (c Catalog) InviteInvalid() string { return c.t("invite.invalid", nil) }
func (c Catalog) InviteFailed() string  { return c.t("invite.failed", nil) }
func (c Catalog) AnotherInvite() string { return c.t("invite.another", nil) }
func (c Catalog) NotAdmin() string      { return c.t("invite.not_admin", nil) }

func (c Catalog) Invite(link string, ttl time.Duration) string {
	return c.t("invite.link", args{"Days": c.count("days", int(ttl.Hours()/24)), "Link": esc(link)})
}

func (c Catalog) ChooseLogin() string    { return c.t("register.choose", nil) }
func (c Catalog) LoginInvalid() string   { return c.t("register.invalid", nil) }
func (c Catalog) RegisterFailed() string { return c.t("register.failed", nil) }
func (c Catalog) HaveAccount() string    { return c.t("register.have_account", nil) }

func (c Catalog) LoginTaken(login string) string {
	return c.t("register.taken", args{"Login": esc(login)})
}

func (c Catalog) Registered(login, password string) string {
	return c.t("register.done", args{"Login": esc(login), "Password": esc(password)})
}

func (c Catalog) NavidromeLinked(login string) string {
	return c.t("navidrome.linked", args{"Login": esc(login)})
}

func (c Catalog) NavidromeNotLinked() string { return c.t("navidrome.not_linked", nil) }
func (c Catalog) Link() string               { return c.t("navidrome.link", nil) }
func (c Catalog) LinkAnother() string        { return c.t("navidrome.link_another", nil) }
func (c Catalog) LinkPrompt() string         { return c.t("navidrome.prompt", nil) }
func (c Catalog) LinkMalformed() string      { return c.t("navidrome.malformed", nil) }
func (c Catalog) LinkTaken() string          { return c.t("navidrome.taken", nil) }
func (c Catalog) LinkWrongPassword() string  { return c.t("navidrome.wrong_password", nil) }
func (c Catalog) LinkFailed() string         { return c.t("navidrome.failed", nil) }

// Linked and LinkedAdmin tell of the songs of Attached Libraries the bot
// sees, if any.
func (c Catalog) Linked(login string, songs int) string {
	return c.withSongs(c.t("navidrome.done", args{"Login": esc(login)}), songs)
}

func (c Catalog) LinkedAdmin(login string, songs int) string {
	return c.withSongs(c.t("navidrome.done_admin", args{"Login": esc(login)}), songs)
}

func (c Catalog) withSongs(text string, songs int) string {
	if songs == 0 {
		return text
	}
	return text + "\n\n" + c.t("navidrome.sees_songs", args{"Songs": c.count("songs", songs)})
}

func (c Catalog) Feed(entries []browse_shared.FeedEntry) string {
	return feed(entries, c.FeedTitle(), c.SharedBy)
}

func (c Catalog) FeedTitle() string { return c.t("feed.title", nil) }

func (c Catalog) FeedScreen() string {
	return "🔗 <b>" + c.FeedTitle() + "</b>\n\n" + c.t("feed.choose", nil)
}

// SharedTrackButton is plain text.
func (c Catalog) SharedTrackButton(entry *browse_shared.FeedEntry) string {
	label := entry.Track.Artist + " — " + entry.Track.Title
	if entry.InLibrary {
		return "✅ " + label
	}
	return label
}

func (c Catalog) SharedTrackCard(entry *browse_shared.FeedEntry) string {
	return c.TrackText(&entry.Track) + "\n\n🔗 " + esc(c.SharedBy(&entry.Author))
}

func (c Catalog) OpenTab(tab string) string { return "✓ " + tab }

func (c Catalog) FeedEmpty() string        { return c.t("feed.empty", nil) }
func (c Catalog) FeedFailed() string       { return c.t("feed.failed", nil) }
func (c Catalog) Taken() string            { return c.t("feed.taken", nil) }
func (c Catalog) AlreadyInLibrary() string { return c.t("feed.already_in_library", nil) }
func (c Catalog) NotShared() string        { return c.t("feed.not_shared", nil) }
func (c Catalog) FileTooLarge() string     { return c.t("feed.file_too_large", nil) }

func (c Catalog) TakeButton() string      { return c.t("feed.take", nil) }
func (c Catalog) SendFileButton() string  { return c.t("feed.send_file", nil) }
func (c Catalog) InLibraryButton() string { return c.t("feed.in_library", nil) }

// SharedBy is plain text: callers escape it for HTML.
func (c Catalog) SharedBy(author *access.User) string {
	return c.t("feed.shared_by", args{"Author": c.authorName(author)})
}

func (c Catalog) authorName(user *access.User) string {
	switch {
	case user == nil:
		return c.t("author.unknown", nil)
	case user.Username != "":
		return "@" + user.Username
	default:
		return c.t("author.no_username", nil)
	}
}

func (c Catalog) Top(t *view_top.Top) string {
	return top(t, topLabels{
		title: c.t("top.title", nil), about: c.t("top.about", nil), shared: c.TopSharedLabel(), taken: c.TopTakenLabel(),
		allTime: c.TopAllTimeLabel(), thisMonth: c.TopThisMonthLabel(), nobody: c.TopNobody(),
	}, c.authorName)
}

func (c Catalog) TopSharedLabel() string    { return c.t("top.shared", nil) }
func (c Catalog) TopTakenLabel() string     { return c.t("top.taken", nil) }
func (c Catalog) TopAllTimeLabel() string   { return c.t("top.all_time", nil) }
func (c Catalog) TopThisMonthLabel() string { return c.t("top.this_month", nil) }
func (c Catalog) TopNobody() string         { return c.t("top.nobody", nil) }

func (c Catalog) TopFailed() string { return c.t("top.failed", nil) }

func (c Catalog) ProviderName(name provider.ProviderName) string {
	return c.providerText(name, "name", string(name))
}

func (c Catalog) ProviderButton(name provider.ProviderName) string {
	return c.providerText(name, "button", "🎵 "+c.ProviderName(name))
}

// providerText reads provider.<name>.<field>, a Provider without it gets
// the fallback.
func (c Catalog) providerText(name provider.ProviderName, field, fallback string) string {
	if text, ok := c.lookup("provider."+string(name)+"."+field, nil, nil); ok {
		return text
	}
	return fallback
}

// providerArgs: Of is the name inside a phrase, where some languages
// inflect it.
func (c Catalog) providerArgs(name provider.ProviderName) args {
	return args{"Name": esc(c.ProviderName(name)), "Of": esc(c.providerText(name, "of", c.ProviderName(name)))}
}

func (c Catalog) ImportSources() string    { return c.t("sources.text", nil) }
func (c Catalog) ImportsButton() string    { return c.t("sources.imports", nil) }
func (c Catalog) Connect() string          { return c.t("sources.connect", nil) }
func (c Catalog) ImportCollection() string { return c.t("sources.import", nil) }
func (c Catalog) Disconnect() string       { return c.t("sources.disconnect", nil) }
func (c Catalog) Reconnect() string        { return c.t("sources.reconnect", nil) }

func (c Catalog) Provider(name provider.ProviderName, status import_collection.SourceStatus) string {
	state := c.t("sources.not_connected", nil)
	switch status {
	case import_collection.SourceConnected:
		state = c.t("sources.connected", nil)
	case import_collection.SourceTokenRejected:
		state = c.t("sources.token_rejected", nil)
	default:
	}
	return c.t("sources.status", with(c.providerArgs(name), "Status", state))
}

func (c Catalog) TokenPrompt(name provider.ProviderName) string {
	text := c.t("sources.token_prompt", c.providerArgs(name))
	if howTo := c.providerText(name, "token_howto", ""); howTo != "" {
		text += "\n\n" + howTo
	}
	return text
}

func (c Catalog) Connected(name provider.ProviderName) string {
	return c.t("sources.connected_notice", c.providerArgs(name))
}

func (c Catalog) TokenInvalid(name provider.ProviderName) string {
	return c.t("sources.token_invalid", c.providerArgs(name))
}

func (c Catalog) NoSubscription(name provider.ProviderName) string {
	return c.t("sources.no_subscription", c.providerArgs(name))
}

func (c Catalog) ProviderDown(name provider.ProviderName) string {
	return c.t("sources.down", c.providerArgs(name))
}

func (c Catalog) NotConnected(name provider.ProviderName) string {
	return c.t("sources.connect_first", c.providerArgs(name))
}

func (c Catalog) Disconnected(name provider.ProviderName) string {
	return c.t("sources.disconnected", c.providerArgs(name))
}

func (c Catalog) TokenRejected(name provider.ProviderName) string {
	return c.t("sources.token_rejected_notice", c.providerArgs(name))
}

func (c Catalog) Plan(name provider.ProviderName, plan *import_collection.Plan) string {
	size := c.t("plan.size", args{"Tracks": c.tracks(plan.Missing), "Size": c.size(plan.MissingBytes)})
	if plan.Missing < plan.Total {
		size = c.t("plan.size_partly", args{
			"Total": c.tracks(plan.Total), "Missing": c.tracks(plan.Missing), "Size": c.size(plan.MissingBytes),
		})
	}
	text := c.t("plan.text", with(c.providerArgs(name), "Size", size))
	if plan.Usage.Quota.Limited() {
		text += "\n\n" + c.t("plan.usage", args{"Free": c.size(plan.Usage.Free()), "Quota": c.quota(plan.Usage.Quota)})
	}
	return text
}

func (c Catalog) NothingToImport(name provider.ProviderName) string {
	return c.t("plan.nothing", c.providerArgs(name))
}

func (c Catalog) AllImported(name provider.ProviderName, total int) string {
	return c.t("plan.all_imported", with(c.providerArgs(name), "Tracks", c.tracks(total)))
}

func (c Catalog) StartImport() string   { return c.t("plan.start", nil) }
func (c Catalog) ImportRunning() string { return c.t("plan.running", nil) }
func (c Catalog) AllInLibrary() string  { return c.t("plan.all_in_library", nil) }

func (c Catalog) Imports(running []import_collection.ImportProgress) string {
	return c.t("imports.title", nil) + "\n\n" +
		imports(running, importLabels{nothing: c.t("imports.nothing", nil), errors: c.t("imports.errors", nil)},
			func(imp import_collection.ImportProgress) string { return c.ProviderName(imp.Provider) })
}

// ImportSummary counts the tracks that did not fit the Quota instead of
// naming them: there may be thousands.
func (c Catalog) ImportSummary(result *import_collection.ImportResult, contact string) string {
	data := c.providerArgs(result.Provider)
	data["Done"], data["Total"] = result.Progress.Done, result.Total
	text := c.t("imports.summary", data) + failedList(result.FailedNames, c.t("imports.failed", nil), c.t("imports.more", nil))
	if n := result.OverQuota; n > 0 {
		text += "\n\n" + c.withContact(c.t("imports.over_quota", args{"Tracks": c.tracks(n)}), esc(contact))
	}
	return text
}

func (c Catalog) UploadFailed(reason ingest.FailureReason) string {
	switch reason {
	case ingest.ReasonUnsupportedFormat:
		return c.t("upload.unsupported_format", nil)
	case ingest.ReasonCorruptFile:
		return c.t("upload.corrupt_file", nil)
	case ingest.ReasonFetchFailed:
		return c.t("upload.fetch_failed", nil)
	default:
		return c.t("upload.internal", nil)
	}
}

// NoRoom is plain text: it goes to replies and callback answers.
func (c Catalog) NoRoom(usage library.Usage, contact string) string {
	return c.withContact(c.t("quota.no_room", c.usageArgs(usage)), contact)
}

func (c Catalog) SharedLibraryFull(contact string) string {
	return c.withContact(c.t("quota.shared_full", nil), contact)
}

func (c Catalog) withContact(text, contact string) string {
	if contact == "" {
		return text
	}
	return text + "\n" + c.t("quota.contact", args{"Contact": contact})
}

func (c Catalog) usageArgs(usage library.Usage) args {
	return args{"Used": c.size(usage.Used), "Quota": c.quota(usage.Quota)}
}

func (c Catalog) quota(q library.Quota) string {
	if !q.Limited() {
		return c.t("quota.unlimited", nil)
	}
	return c.size(int64(q))
}

// sizeUnits are binary; the language names each in units.<key>, several
// names apart by commas.
var sizeUnits = []struct {
	key   string
	bytes int64
}{{"kb", 1 << 10}, {"mb", 1 << 20}, {"gb", 1 << 30}, {"tb", 1 << 40}}

// ParseSize reads a size the user typed, like "25 ГБ" or "1.5GB"; false
// for anything else, zero and negative sizes too.
func (c Catalog) ParseSize(text string) (int64, bool) {
	text = strings.TrimSpace(text)
	cut := strings.IndexFunc(text, unicode.IsLetter)
	if cut <= 0 {
		return 0, false
	}
	number, err := strconv.ParseFloat(strings.Replace(strings.TrimSpace(text[:cut]), ",", ".", 1), 64)
	unit, ok := c.unitBytes(text[cut:])
	bytes := int64(number * float64(unit))
	if err != nil || !ok || bytes < 1 {
		return 0, false
	}
	return bytes, true
}

func (c Catalog) unitBytes(name string) (int64, bool) {
	for _, unit := range sizeUnits {
		for _, known := range strings.Split(c.t("units."+unit.key, nil), ",") {
			if strings.EqualFold(strings.TrimSpace(known), name) {
				return unit.bytes, true
			}
		}
	}
	return 0, false
}

// size rounds to a tenth of its unit.
func (c Catalog) size(n int64) string {
	key, unit := "size.kb", int64(1<<10)
	switch {
	case n >= 1<<30:
		key, unit = "size.gb", 1<<30
	case n >= 1<<20:
		key, unit = "size.mb", 1<<20
	}
	return c.t(key, args{"N": c.decimal(float64(n) / float64(unit))})
}

// decimal drops a zero tenth.
func (c Catalog) decimal(x float64) string {
	text := strconv.FormatFloat(math.Round(x*10)/10, 'f', -1, 64)
	return strings.Replace(text, ".", c.t("size.decimal_separator", nil), 1)
}

func (c Catalog) StoredInInbox() string { return c.t("upload.inbox", nil) }
func (c Catalog) AlreadyExists() string { return c.t("upload.already_exists", nil) }

func (c Catalog) InboxNotShareable() string { return c.t("share.inbox", nil) }
func (c Catalog) NotOwnTrack() string       { return c.t("share.not_own", nil) }
func (c Catalog) Unshared() string          { return c.t("share.unshared", nil) }

func (c Catalog) ShareResult(result *share_tracks.ShareResult) string {
	switch {
	case result.Created == 0 && result.AlreadyShared == 0:
		return c.t("share.already", nil)
	case result.AlreadyShared == 0:
		return c.t("share.created", args{"Tracks": c.tracks(result.Created)})
	case result.Created == 0:
		return c.t("share.by_other", args{"Author": c.authorName(result.Author)})
	default:
		return c.t("share.created_partly", args{"Tracks": c.tracks(result.Created), "Already": c.tracks(result.AlreadyShared)})
	}
}

func (c Catalog) NowPlaying(track *show_playing.NowPlaying) string {
	return nowPlaying(track, nowPlayingLabels{playing: c.t("playing.now", nil), paused: c.t("playing.paused", nil)}, c.playingDetails(track))
}

// playingDetails falls back to the album Navidrome tells for a song the
// user's libraries do not hold.
func (c Catalog) playingDetails(track *show_playing.NowPlaying) string {
	if track.Track != nil {
		return c.AudioCaption(track.Track)
	}
	if track.Album == "" {
		return ""
	}
	return "💿 " + esc(track.Album)
}

func (c Catalog) NowPlayingArticle(track *show_playing.NowPlaying) Article {
	return nowPlayingArticle(track, c.NowPlaying(track))
}

// NotSentCaption replaces PendingCaption when the file cannot be sent.
func (c Catalog) NotSentCaption(caption string, tooLarge bool) string {
	return caption + "\n" + c.NotSentNote(tooLarge)
}

func (c Catalog) AlbumArticle(album repositories.AlbumSummary) Article {
	return Article{
		Title:       AlbumTitle(album.AlbumArtist, album.Album),
		Description: c.AlbumDescription(album),
		Message:     "⏳ " + AlbumCaption(album.AlbumArtist, album.Album),
	}
}

func (c Catalog) AlbumLinkCaption(album repositories.AlbumSummary, url string) string {
	return c.AlbumText(album) + "\n" + c.listenLink(url)
}

func (c Catalog) ListenLinkCaption(caption, url string) string {
	return caption + "\n" + c.listenLink(url)
}

func (c Catalog) listenLink(url string) string {
	return fmt.Sprintf(`🔗 <a href="%s">%s</a>`, esc(url), esc(c.t("listen_link.open", nil)))
}

func (c Catalog) NotIndexedCaption(caption string) string {
	return caption + "\n⏳ " + esc(c.t("listen_link.not_ready", nil))
}

func (c Catalog) LinkFailedCaption(caption string) string {
	return caption + "\n⚠️ " + esc(c.t("listen_link.failed", nil))
}

func (c Catalog) NotSentNote(tooLarge bool) string {
	if tooLarge {
		return "⚠️ " + esc(c.FileTooLarge())
	}
	return "⚠️ " + esc(c.t("playing.file_not_sent", nil))
}

func (c Catalog) NothingPlaying() Article   { return c.article("playing.nothing") }
func (c Catalog) NowPlayingFailed() Article { return c.article("playing.failed") }
func (c Catalog) HistoryEmpty() Article     { return c.article("recent.empty") }
func (c Catalog) HistoryFailed() Article    { return c.article("recent.failed") }

func (c Catalog) RecentList(tracks []show_playing.RecentTrack, now time.Time) Article {
	count := args{"Tracks": c.count("played_tracks", len(tracks))}
	return Article{
		Title:       c.t("recent.title", count),
		Description: c.t("recent.about", nil),
		Message: recentList(tracks, c.t("recent.header", count), func(t *show_playing.RecentTrack) string {
			return c.ago(t.PlayedAt, now)
		}),
	}
}

func (c Catalog) RecentDescription(track *show_playing.RecentTrack, now time.Time) string {
	return fmt.Sprintf("%s · %s", track.Album, c.ago(track.PlayedAt, now))
}

func (c Catalog) NoNavidromeAccount(bot string) Article {
	return Article{
		Title:       c.t("no_account.title", nil),
		Description: c.t("no_account.about", nil),
		Message:     c.t("no_account.message", args{"Bot": esc(bot)}),
	}
}

func (c Catalog) FeedList(entries []browse_shared.FeedEntry) Article {
	return Article{
		Title:       c.t("feed.list_title", args{"Tracks": c.tracks(len(entries))}),
		Description: c.t("feed.list_description", nil),
		Message:     c.Feed(entries),
	}
}

func (c Catalog) FeedCaption(track *library.Track, author *access.User) string {
	return joinParts("\n", c.AudioCaption(track), "🔗 "+esc(c.SharedBy(author)))
}

func (c Catalog) FeedText(track *library.Track, author *access.User) string {
	return c.TrackText(track) + "\n🔗 " + esc(c.SharedBy(author))
}

func (c Catalog) FeedEmptyArticle() Article {
	return Article{Title: c.t("feed.empty_title", nil), Description: c.t("feed.empty_description", nil), Message: c.FeedEmpty()}
}

func (c Catalog) FeedFailedArticle() Article { return c.failedArticle("feed") }

func (c Catalog) TopArticle(t *view_top.Top) Article {
	return Article{Title: "🏆 " + c.t("top.title", nil), Description: c.t("top.about", nil), Message: c.Top(t)}
}

func (c Catalog) TopFailedArticle() Article { return c.failedArticle("top") }

// failedArticle reads <group>.failed_title and _description and sends the
// description as a warning.
func (c Catalog) failedArticle(group string) Article {
	failed := c.t(group+".failed_description", nil)
	return Article{Title: c.t(group+".failed_title", nil), Description: failed, Message: "⚠️ " + failed}
}

func (c Catalog) Hints(bot string) []Hint {
	hints := make([]Hint, 0, len(inlineCommands))
	for _, command := range inlineCommands {
		hints = append(hints, Hint{command.name, Article{
			Title:       command.icon + " " + command.name,
			Description: c.t("hint."+command.name+".about", nil),
			Message:     command.icon + " " + inlineUsage(bot, command.name) + " — " + c.t("hint."+command.name+".message", nil),
		}})
	}
	return hints
}

func (c Catalog) Try() string { return c.t("hint.try", nil) }

// SearchHint follows the Hints: any other text searches.
func (c Catalog) SearchHint(bot string) Article {
	return Article{
		Title:       c.t("search.hint_title", nil),
		Description: c.t("search.hint_about", nil),
		Message:     c.t("search.hint_message", args{"Usage": inlineUsage(bot, "queen")}),
	}
}

func (c Catalog) NothingFound() Article        { return c.article("search.nothing") }
func (c Catalog) SearchFailedArticle() Article { return c.failedArticle("search") }

// article reads <prefix>_title, _description and _message.
func (c Catalog) article(prefix string) Article {
	return Article{
		Title:       c.t(prefix+"_title", nil),
		Description: c.t(prefix+"_description", nil),
		Message:     c.t(prefix+"_message", nil),
	}
}

func (c Catalog) tracks(n int) string { return c.count("tracks", n) }

func (c Catalog) ago(t, now time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return c.t("ago.now", nil)
	case d < time.Hour:
		return c.t("ago.minutes", args{"N": int(d.Minutes())})
	case d < 24*time.Hour:
		return c.t("ago.hours", args{"N": int(d.Hours())})
	default:
		return c.t("ago.days", args{"N": int(d.Hours() / 24)})
	}
}

// t shows a text missing everywhere as its key.
func (c Catalog) t(id string, data args) string {
	text, _ := c.lookup(id, data, nil)
	return text
}

func (c Catalog) count(id string, n int) string {
	text, _ := c.lookup(id, args{"Count": n}, n)
	return text
}

func (c Catalog) lookup(id string, data args, pluralCount any) (string, bool) {
	all := args{"Service": esc(c.brand.Service), "NavidromeURL": esc(c.brand.NavidromeURL)}
	for k, v := range data {
		all[k] = v
	}
	text, err := c.loc.Localize(&goi18n.LocalizeConfig{MessageID: id, TemplateData: all, PluralCount: pluralCount})
	var notFound *goi18n.MessageNotFoundErr
	switch {
	case err == nil:
		return text, true
	case errors.As(err, &notFound) && text != "":
		// The language lacks the text; it comes in English.
		return text, true
	case errors.As(err, &notFound):
		return id, false
	default:
		slog.Error("render_text", "language", c.lang, "key", id, "error", err)
		return id, false
	}
}

func with(data args, key string, value any) args {
	data[key] = value
	return data
}
