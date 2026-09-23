package telegram

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/lubaskinc0de/navidrome-tg/internal/application"
	"github.com/lubaskinc0de/navidrome-tg/internal/config"
	"github.com/lubaskinc0de/navidrome-tg/internal/database"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
	tgprovider "github.com/lubaskinc0de/navidrome-tg/internal/providers/telegram"
)

const recentTracksLimit = 10

type Handler struct {
	enqueueIngest     *application.EnqueueIngest
	getNowPlaying     *application.GetNowPlaying
	getRecentlyPlayed *application.GetRecentlyPlayed
	users             *database.UserRepository
	config            config.Config
}

func NewHandler(
	enqueueIngest *application.EnqueueIngest,
	nowPlaying *application.GetNowPlaying,
	recentlyPlayed *application.GetRecentlyPlayed,
	users *database.UserRepository,
	config config.Config,
) *Handler {
	return &Handler{
		enqueueIngest:     enqueueIngest,
		users:             users,
		config:            config,
		getNowPlaying:     nowPlaying,
		getRecentlyPlayed: recentlyPlayed,
	}
}

func (h *Handler) HandleAudio(
	ctx context.Context,
	b *bot.Bot,
	update *models.Update,
) {
	file, mime, ok := audioFile(update.Message)
	if !ok {
		return
	}
	msg := application.MessageRef{ChatID: update.Message.Chat.ID, MessageID: update.Message.ID}

	format, ok := domain.FormatOf(file.Name, mime)
	if !ok {
		slog.Info("unsupported_format", "file_name", file.Name, "mime_type", mime)
		reject(ctx, b, msg, application.ReasonUnsupportedFormat)
		return
	}
	file.Format = format

	ref, err := tgprovider.Ref(file)
	if err != nil {
		slog.Error("track_ref", "error", err)
		reject(ctx, b, msg, application.ReasonInternal)
		return
	}

	// 👀 goes first: a worker may finish and set 👍 before Execute returns.
	setReaction(ctx, b, msg, "👀")
	err = h.enqueueIngest.Execute(ctx, application.IngestRequest{Ref: ref, Message: msg})
	if err != nil {
		slog.Error("enqueue_ingest", "error", err)
		reject(ctx, b, msg, application.ReasonInternal)
	}
}

func HasAudio(update *models.Update) bool {
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
			Kind:      domain.TelegramFileAudio,
			Name:      msg.Audio.FileName,
			Performer: msg.Audio.Performer,
			Title:     msg.Audio.Title,
		}, msg.Audio.MimeType, true
	case msg.Document != nil:
		return tgprovider.File{
			ID:       msg.Document.FileID,
			UniqueID: msg.Document.FileUniqueID,
			Kind:     domain.TelegramFileDocument,
			Name:     msg.Document.FileName,
		}, msg.Document.MimeType, true
	}
	return tgprovider.File{}, "", false
}

func (h *Handler) HandleInlineQuery(
	ctx context.Context,
	b *bot.Bot,
	update *models.Update,
) {
	if update.InlineQuery == nil {
		return
	}

	query := strings.TrimSpace(
		strings.ToLower(update.InlineQuery.Query),
	)

	switch query {
	case "", "np":
		h.handleNowPlaying(ctx, b, update)
	case "recent", "last":
		h.handleRecentlyPlayed(ctx, b, update)
	default:
		answerInlineArticle(
			ctx,
			b,
			update.InlineQuery.ID,
			"unknown-command",
			"❓ Неизвестная команда",
			"Доступно: np — текущий трек, recent — последние треки",
			"❓ Неизвестная команда. Доступно: np, recent",
		)
	}
}

func (h *Handler) HandleCallbackQuery(
	ctx context.Context,
	b *bot.Bot,
	update *models.Update,
) {
	if update.CallbackQuery == nil {
		return
	}

	_, err := b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
	})
	if err != nil {
		slog.Error("answer_callback_query", "error", err)
	}
}

func (h *Handler) handleNowPlaying(
	ctx context.Context,
	b *bot.Bot,
	update *models.Update,
) {
	if update.InlineQuery == nil {
		return
	}

	track, err := h.getNowPlaying.Execute(ctx)
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

	if track.TelegramFile != nil {
		answerInline(ctx, b, update.InlineQuery.ID, cachedFileResult(track.ID, track.TelegramFile, text))
		return
	}

	answerInlineArticle(
		ctx,
		b,
		update.InlineQuery.ID,
		track.ID,
		fmt.Sprintf("🎧 %s — %s", track.Artist, track.Title),
		description,
		text,
	)
}

func (h *Handler) handleRecentlyPlayed(
	ctx context.Context,
	b *bot.Bot,
	update *models.Update,
) {
	if update.InlineQuery == nil {
		return
	}

	tracks, err := h.getRecentlyPlayed.Execute(ctx, recentTracksLimit)
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

		if track.TelegramFile != nil {
			results = append(results, cachedFileResult("recent-"+track.ID, track.TelegramFile, caption))
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

func recentTracksText(tracks []application.RecentTrack, now time.Time) string {
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

func nowPlayingText(track *application.NowPlaying) string {
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

func cachedFileResult(id string, file *domain.TelegramFile, caption string) models.InlineQueryResult {
	if file.Kind == domain.TelegramFileDocument {
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
