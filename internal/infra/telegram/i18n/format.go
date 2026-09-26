package i18n

import (
	"fmt"
	"html"
	"math"
	"strings"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/browse_shared"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/import_collection"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/ingest_track"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/show_playing"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/view_top"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
)

var esc = html.EscapeString

// footer is built in code so a translation cannot drop the source link or
// the author.
func footer(source, author string) string {
	return fmt.Sprintf(`<a href="%s">%s</a> · %s %s`, SourceURL, esc(source), esc(author), Author)
}

// inlineCommands are the commands of inline mode, as Home and the hints
// list them.
var inlineCommands = []struct{ name, icon string }{
	{"np", "🎧"}, {"recent", "📜"}, {"shared", "🔗"}, {"top", "🏆"},
}

func inlineUsage(bot, command string) string {
	return fmt.Sprintf("<code>@%s %s</code>", esc(bot), command)
}

// ResultTitle is how an inline result shows a track; it is plain text.
func ResultTitle(artist, title string) string {
	return "🎧 " + artist + " — " + title
}

// TrackCaption is how a sent track is captioned.
func TrackCaption(artist, title string) string {
	return "🎧 " + trackLine(artist, title)
}

func formatSeconds(seconds int) string {
	return fmt.Sprintf("%02d:%02d", seconds/60, seconds%60)
}

func playbackBar(position, duration int) string {
	const segments = 12

	filled := 0
	if duration > 0 {
		filled = position * segments / duration
	}
	filled = min(max(filled, 0), segments)
	return strings.Repeat("━", filled) + "●" + strings.Repeat("─", segments-filled)
}

type nowPlayingLabels struct {
	playing, paused string
}

func nowPlaying(track *show_playing.NowPlaying, labels nowPlayingLabels) string {
	position := track.PositionMs / 1000
	icon, header := "▶️", labels.playing
	if strings.EqualFold(track.State, "paused") {
		icon, header = "⏸", labels.paused
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s <b>%s:</b>\n\n", icon, header)
	fmt.Fprintf(&b, "🎧 <b>%s</b>\n", esc(track.Title))
	fmt.Fprintf(&b, "👤 %s\n", esc(track.Artist))
	if track.Album != "" {
		fmt.Fprintf(&b, "💿 <i>%s</i>\n", esc(track.Album))
	}
	fmt.Fprintf(&b, "\n<code>%s</code>  %s / %s",
		playbackBar(position, track.Duration), formatSeconds(position), formatSeconds(track.Duration))
	return b.String()
}

func nowPlayingArticle(track *show_playing.NowPlaying, message string) Article {
	return Article{
		Title: ResultTitle(track.Artist, track.Title),
		Description: fmt.Sprintf("%s · %s / %s",
			track.Album, formatSeconds(track.PositionMs/1000), formatSeconds(track.Duration)),
		Message: message,
	}
}

func recentList(tracks []show_playing.RecentTrack, header string, ago func(*show_playing.RecentTrack) string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "📜 <b>%s:</b>\n\n", header)
	for i := range tracks {
		track := &tracks[i]
		fmt.Fprintf(&b, "%d. %s\n     <i>%s · %s</i>\n",
			i+1, trackLine(track.Artist, track.Title), formatSeconds(track.Duration), ago(track))
	}
	return b.String()
}

func trackLine(artist, title string) string {
	return fmt.Sprintf("<b>%s</b> — %s", esc(artist), esc(title))
}

func feed(entries []browse_shared.FeedEntry, header string, sharedBy func(*access.User) string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "🔗 <b>%s:</b>\n\n", header)
	for i := range entries {
		fmt.Fprintf(&b, "%d. %s\n     <i>%s</i>\n", i+1, trackLine(entries[i].Track.Artist, entries[i].Track.Title), esc(sharedBy(&entries[i].Author)))
	}
	return b.String()
}

type topLabels struct {
	title, shared, taken, allTime, thisMonth, nobody string
}

func top(t *view_top.Top, labels topLabels, name func(*access.User) string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "🏆 <b>%s</b>", labels.title)
	rating := func(title, period string, entries []repositories.TopEntry) {
		fmt.Fprintf(&b, "\n\n<b>%s</b>, %s:", title, period)
		if len(entries) == 0 {
			b.WriteString("\n")
			b.WriteString(labels.nobody)
			return
		}
		for i := range entries {
			fmt.Fprintf(&b, "\n%d. %s — %d", i+1, esc(name(&entries[i].User)), entries[i].Count)
		}
	}
	rating("🔗 "+labels.shared, labels.allTime, t.Sharers.AllTime)
	rating("🔗 "+labels.shared, labels.thisMonth, t.Sharers.ThisMonth)
	rating("⭐ "+labels.taken, labels.allTime, t.TakenAuthors.AllTime)
	rating("⭐ "+labels.taken, labels.thisMonth, t.TakenAuthors.ThisMonth)
	return b.String()
}

const importCells = 10

// importBar gives any failure at least one red cell.
func importBar(total int, progress repositories.BatchProgress) string {
	if total <= 0 {
		return strings.Repeat("⬜", importCells)
	}
	cells := func(n int) int {
		return int(math.Round(float64(n) * importCells / float64(total)))
	}
	failed := cells(progress.Failed)
	if progress.Failed > 0 {
		failed = max(failed, 1)
	}
	failed = min(failed, importCells)
	done := min(cells(progress.Done), importCells-failed)
	return strings.Repeat("🟩", done) + strings.Repeat("🟥", failed) + strings.Repeat("⬜", importCells-done-failed)
}

type importLabels struct {
	nothing, errors string
}

func imports(running []import_collection.RunningImport, labels importLabels, name func(import_collection.RunningImport) string) string {
	if len(running) == 0 {
		return labels.nothing
	}
	var b strings.Builder
	for n, imp := range running {
		if n > 0 {
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "📥 <b>%s</b>\n%s %d / %d", esc(name(imp)), importBar(imp.Total, imp.Progress), imp.Progress.Done, imp.Total)
		if imp.Progress.Failed > 0 {
			fmt.Fprintf(&b, " · %s %d", labels.errors, imp.Progress.Failed)
		}
	}
	return b.String()
}

// maxFailedListed keeps a summary within Telegram's 4096 characters.
const maxFailedListed = 40

func failedList(state *ingest_track.BatchState, header, more string) string {
	failed := state.FailedNames
	if len(failed) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\n\n❌ %s (%d):", header, len(failed))
	for _, name := range failed[:min(len(failed), maxFailedListed)] {
		fmt.Fprintf(&b, "\n• %s", esc(name))
	}
	if len(failed) > maxFailedListed {
		fmt.Fprintf(&b, "\n%s %d", more, len(failed)-maxFailedListed)
	}
	return b.String()
}
