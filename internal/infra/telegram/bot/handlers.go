package bot

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/add_track"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/browse_shared"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/connect_navidrome"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/connect_provider"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/greet_stranger"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/import_collection"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/invite_friend"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/join_by_invite"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/share_tracks"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/show_playing"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/view_top"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/ingest"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
	tgprovider "github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/provider"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/store"
)

const recentTracksLimit = 10

type Handler struct {
	Zvuk provider.ProviderName

	EnqueueIngest             *add_track.EnqueueIngest
	GetNowPlaying             *show_playing.GetNowPlaying
	GetRecentlyPlayed         *show_playing.GetRecentlyPlayed
	LinkNavidromeAccount      *connect_navidrome.LinkNavidromeAccount
	CreateInvite              *invite_friend.CreateInvite
	CheckCanInvite            *invite_friend.CheckCanInvite
	AcceptInvite              *join_by_invite.AcceptInvite
	RegisterAccount           *connect_navidrome.RegisterNavidromeAccount
	ShowShareOptions          *share_tracks.ShowShareOptions
	ShareTrack                *share_tracks.ShareTrack
	ShareAlbum                *share_tracks.ShareAlbum
	UnshareTrack              *share_tracks.UnshareTrack
	UnshareAlbum              *share_tracks.UnshareAlbum
	ViewFeed                  *browse_shared.ViewFeed
	TakeTrack                 *browse_shared.TakeTrack
	GetTrackAudio             *browse_shared.GetTrackAudio
	GetTop                    *view_top.GetTop
	GetServiceStats           *greet_stranger.GetServiceStats
	ConnectProviderAccount    *connect_provider.ConnectProviderAccount
	DisconnectProviderAccount *connect_provider.DisconnectProviderAccount
	PlanImport                *import_collection.PlanImport
	StartImport               *import_collection.StartImport
	AdminContact              string

	Dialogs       *store.Dialogs
	JobMessages   *store.JobMessages
	BatchMessages *store.BatchMessages
	Files         *store.Files
	Sender        *AudioSender
}

func (h *Handler) handleAudio(
	ctx context.Context,
	b *bot.Bot,
	update *models.Update,
) {
	file, mime, ok := audioFile(update.Message)
	if !ok {
		return
	}
	msg := messageRef{chatID: update.Message.Chat.ID, messageID: update.Message.ID}

	format, ok := library.FormatOf(file.Name, mime)
	if !ok {
		slog.Info("unsupported_format", "file_name", file.Name, "mime_type", mime)
		reject(ctx, b, msg, failureTexts[ingest.ReasonUnsupportedFormat])
		return
	}
	file.Format = format

	ref, err := tgprovider.Ref(file)
	if err != nil {
		slog.Error("track_ref", "error", err)
		reject(ctx, b, msg, failureTexts[ingest.ReasonInternal])
		return
	}

	// 👀 goes first: the Poller may set 👍 as soon as the job is remembered.
	setReaction(ctx, b, msg, "👀")
	jobID, err := h.EnqueueIngest.Execute(ctx, ref)
	if err != nil {
		slog.Error("enqueue_ingest", "error", err)
		reject(ctx, b, msg, failureTexts[ingest.ReasonInternal])
		return
	}
	err = h.JobMessages.Remember(ctx, store.JobMessage{
		JobID: jobID, ChatID: msg.chatID, MessageID: msg.messageID,
		FileID: file.ID, FileUniqueID: file.UniqueID, FileKind: file.Kind,
	})
	if err != nil {
		slog.Error("remember_job_message", "job_id", jobID, "error", err)
	}
}

func hasAudio(update *models.Update) bool {
	_, _, ok := audioFile(update.Message)
	return ok
}

func audioFile(msg *models.Message) (file tgprovider.File, mimeType string, ok bool) {
	switch {
	case msg == nil:
		return tgprovider.File{}, "", false
	case msg.Audio != nil:
		return tgprovider.File{
			ID:        msg.Audio.FileID,
			UniqueID:  msg.Audio.FileUniqueID,
			Kind:      store.FileAudio,
			Name:      msg.Audio.FileName,
			Performer: msg.Audio.Performer,
			Title:     msg.Audio.Title,
		}, msg.Audio.MimeType, true
	case msg.Document != nil:
		return tgprovider.File{
			ID:       msg.Document.FileID,
			UniqueID: msg.Document.FileUniqueID,
			Kind:     store.FileDocument,
			Name:     msg.Document.FileName,
		}, msg.Document.MimeType, true
	}
	return tgprovider.File{}, "", false
}

func (h *Handler) handleInlineQuery(
	ctx context.Context,
	b *bot.Bot,
	update *models.Update,
) {
	query := strings.TrimSpace(
		strings.ToLower(update.InlineQuery.Query),
	)

	switch query {
	case "", "np":
		h.handleNowPlaying(ctx, b, update)
	case "recent", "last":
		h.handleRecentlyPlayed(ctx, b, update)
	case "shared":
		h.handleInlineFeed(ctx, b, update)
	case "top":
		h.handleInlineTop(ctx, b, update)
	default:
		answerInlineArticle(
			ctx,
			b,
			update.InlineQuery.ID,
			"unknown-command",
			"❓ Неизвестная команда",
			"Доступно: np — текущий трек, recent — последние треки, shared — расшаренное, top — рейтинг",
			"❓ Неизвестная команда. Доступно: np, recent, shared, top",
		)
	}
}

func (h *Handler) handleNowPlaying(
	ctx context.Context,
	b *bot.Bot,
	update *models.Update,
) {
	track, err := h.GetNowPlaying.Execute(ctx)
	if errors.Is(err, repositories.ErrNavidromeAccountNotFound) {
		answerNoNavidromeAccount(ctx, b, update.InlineQuery.ID)
		return
	}
	if err != nil {
		slog.Error("get_now_playing", "error", err)

		answerInlineArticle(
			ctx,
			b,
			update.InlineQuery.ID,
			"error",
			"⚠️ Navidrome недоступен",
			"Не удалось получить текущий трек",
			"⚠️ Не удалось получить текущий трек",
		)
		return
	}

	if track == nil {
		answerInlineArticle(
			ctx,
			b,
			update.InlineQuery.ID,
			"nothing-playing",
			"💤 Ничего не играет",
			"Сейчас в Navidrome ничего не воспроизводится",
			"💤 Сейчас ничего не играет",
		)
		return
	}

	position := formatSeconds(track.PositionMs / 1000)
	duration := formatSeconds(track.Duration)
	text := nowPlayingText(track)

	description := fmt.Sprintf(
		"%s · %s / %s",
		track.Album,
		position,
		duration,
	)

	var markup models.ReplyMarkup
	if track.ShareableTrackID != 0 {
		markup = &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{{
			{Text: "🔗 Share", CallbackData: callbackData(actionShareTrack, track.ShareableTrackID)},
		}}}
	}

	if file := h.fileOf(ctx, track.Track); file != nil {
		answerInline(ctx, b, update.InlineQuery.ID, withMarkup(cachedFileResult(track.ID, file, text), markup))
		return
	}

	answerInline(ctx, b, update.InlineQuery.ID, &models.InlineQueryResultArticle{
		ID:          track.ID,
		Title:       fmt.Sprintf("🎧 %s — %s", track.Artist, track.Title),
		Description: description,
		InputMessageContent: &models.InputTextMessageContent{
			MessageText: text,
			ParseMode:   models.ParseModeHTML,
		},
		ReplyMarkup: markup,
	})
}

func (h *Handler) handleRecentlyPlayed(
	ctx context.Context,
	b *bot.Bot,
	update *models.Update,
) {
	tracks, err := h.GetRecentlyPlayed.Execute(ctx, recentTracksLimit)
	if errors.Is(err, repositories.ErrNavidromeAccountNotFound) {
		answerNoNavidromeAccount(ctx, b, update.InlineQuery.ID)
		return
	}
	if err != nil {
		slog.Error("get_recently_played", "error", err)

		answerInlineArticle(
			ctx,
			b,
			update.InlineQuery.ID,
			"error",
			"⚠️ Navidrome недоступен",
			"Не удалось получить историю прослушиваний",
			"⚠️ Не удалось получить историю прослушиваний",
		)
		return
	}

	if len(tracks) == 0 {
		answerInlineArticle(
			ctx,
			b,
			update.InlineQuery.ID,
			"nothing-played",
			"💤 История пуста",
			"В Navidrome ещё ничего не прослушано",
			"💤 История прослушиваний пуста",
		)
		return
	}

	now := time.Now()
	results := make([]models.InlineQueryResult, 0, len(tracks)+1)

	results = append(results, &models.InlineQueryResultArticle{
		ID:          "recent-list",
		Title:       fmt.Sprintf("📜 Последние %d треков", len(tracks)),
		Description: "Отправить весь список одним сообщением",
		InputMessageContent: &models.InputTextMessageContent{
			MessageText: recentTracksText(tracks, now),
			ParseMode:   models.ParseModeHTML,
		},
	})

	for _, track := range tracks {
		caption := fmt.Sprintf(
			"🎧 <b>%s</b> — %s",
			html.EscapeString(track.Artist),
			html.EscapeString(track.Title),
		)

		if file := h.fileOf(ctx, track.Track); file != nil {
			results = append(results, cachedFileResult("recent-"+track.ID, file, caption))
			continue
		}

		results = append(results, &models.InlineQueryResultArticle{
			ID:          "recent-" + track.ID,
			Title:       fmt.Sprintf("🎧 %s — %s", track.Artist, track.Title),
			Description: fmt.Sprintf("%s · %s", track.Album, timeAgo(track.PlayedAt, now)),
			InputMessageContent: &models.InputTextMessageContent{
				MessageText: caption,
				ParseMode:   models.ParseModeHTML,
			},
		})
	}

	answerInline(ctx, b, update.InlineQuery.ID, results...)
}

func recentTracksText(tracks []show_playing.RecentTrack, now time.Time) string {
	var b strings.Builder

	fmt.Fprintf(&b, "📜 <b>Последние %d треков:</b>\n\n", len(tracks))

	for i, track := range tracks {
		fmt.Fprintf(
			&b,
			"%d. <b>%s</b> — %s\n     <i>%s · %s</i>\n",
			i+1,
			html.EscapeString(track.Artist),
			html.EscapeString(track.Title),
			formatSeconds(track.Duration),
			timeAgo(track.PlayedAt, now),
		)
	}

	return b.String()
}

func timeAgo(t time.Time, now time.Time) string {
	d := now.Sub(t)

	switch {
	case d < time.Minute:
		return "только что"
	case d < time.Hour:
		return fmt.Sprintf("%d мин назад", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d ч назад", int(d.Hours()))
	default:
		return fmt.Sprintf("%d дн назад", int(d.Hours()/24))
	}
}

func nowPlayingText(track *show_playing.NowPlaying) string {
	position := track.PositionMs / 1000
	icon := "▶️"
	header := "Сейчас играет"
	if strings.EqualFold(track.State, "paused") {
		icon = "⏸"
		header = "На паузе"
	}

	var b strings.Builder

	fmt.Fprintf(&b, "%s <b>%s:</b>\n\n", icon, header)
	fmt.Fprintf(&b, "🎧 <b>%s</b>\n", html.EscapeString(track.Title))
	fmt.Fprintf(&b, "👤 %s\n", html.EscapeString(track.Artist))

	if track.Album != "" {
		fmt.Fprintf(&b, "💿 <i>%s</i>\n", html.EscapeString(track.Album))
	}

	fmt.Fprintf(
		&b,
		"\n<code>%s</code>  %s / %s",
		progressBar(position, track.Duration),
		formatSeconds(position),
		formatSeconds(track.Duration),
	)

	return b.String()
}

func progressBar(position, duration int) string {
	const segments = 12

	filled := 0
	if duration > 0 {
		filled = position * segments / duration
	}
	filled = min(max(filled, 0), segments)

	return strings.Repeat("━", filled) + "●" + strings.Repeat("─", segments-filled)
}

func answerInlineArticle(
	ctx context.Context,
	b *bot.Bot,
	inlineQueryID string,
	id string,
	title string,
	description string,
	message string,
) {
	answerInline(ctx, b, inlineQueryID, &models.InlineQueryResultArticle{
		ID:          id,
		Title:       title,
		Description: description,
		InputMessageContent: &models.InputTextMessageContent{
			MessageText: message,
			ParseMode:   models.ParseModeHTML,
		},
	})
}

func cachedFileResult(id string, file *store.File, caption string) models.InlineQueryResult {
	if file.Kind == store.FileDocument {
		return &models.InlineQueryResultCachedDocument{
			ID:             id,
			Title:          "🎧",
			DocumentFileID: file.ID,
			Caption:        caption,
			ParseMode:      models.ParseModeHTML,
		}
	}
	return &models.InlineQueryResultCachedAudio{
		ID:          id,
		AudioFileID: file.ID,
		Caption:     caption,
		ParseMode:   models.ParseModeHTML,
	}
}

func withMarkup(result models.InlineQueryResult, markup models.ReplyMarkup) models.InlineQueryResult {
	switch r := result.(type) {
	case *models.InlineQueryResultCachedAudio:
		r.ReplyMarkup = markup
	case *models.InlineQueryResultCachedDocument:
		r.ReplyMarkup = markup
	}
	return result
}

func answerInline(
	ctx context.Context,
	b *bot.Bot,
	inlineQueryID string,
	results ...models.InlineQueryResult,
) {
	_, err := b.AnswerInlineQuery(ctx, &bot.AnswerInlineQueryParams{
		InlineQueryID: inlineQueryID,
		Results:       results,
		CacheTime:     1,
		IsPersonal:    true,
	})
	if err != nil {
		slog.Error("answer_inline_query", "error", err)
	}
}

func formatSeconds(seconds int) string {
	minutes := seconds / 60
	seconds %= 60

	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}
