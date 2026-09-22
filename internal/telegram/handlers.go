package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/lubaskinc0de/navidrome-tg/internal/application"
	"github.com/lubaskinc0de/navidrome-tg/internal/config"
	"github.com/lubaskinc0de/navidrome-tg/internal/database"
)

type Handler struct {
	saveTrack     *application.SaveTrack
	getNowPlaying *application.GetNowPlaying
	users         *database.UserRepository
	config        config.Config
}

func NewHandler(
	saveTrack *application.SaveTrack,
	nowPlaying *application.GetNowPlaying,
	users *database.UserRepository,
	config config.Config,
) *Handler {
	return &Handler{
		saveTrack:     saveTrack,
		users:         users,
		config:        config,
		getNowPlaying: nowPlaying,
	}
}

func (h *Handler) HandleTrack(
	ctx context.Context,
	b *bot.Bot,
	update *models.Update,
) {
	if update.Message == nil || update.Message.Audio == nil {
		return
	}
	audio := update.Message.Audio
	file, err := b.GetFile(ctx, &bot.GetFileParams{
		FileID: audio.FileID,
	})
	if err != nil {
		slog.Error("get_file", "error", err)
		return
	}
	url := b.FileDownloadLink(file)

	setReaction(ctx, b, update, "👀")
	err = h.saveTrack.Execute(
		ctx,
		application.TrackData{
			TelegramFileID:       audio.FileID,
			TelegramFileUniqueID: audio.FileUniqueID,

			FileName: audio.FileName,
			MimeType: audio.MimeType,
			FileSize: audio.FileSize,
			Duration: audio.Duration,

			Title:     audio.Title,
			Performer: audio.Performer,

			MessageId:   update.Message.ID,
			DownloadUrl: url,
		},
		h.config.MusicDir,
	)

	if err != nil {
		slog.Error("save_track", "error", err)
		setReaction(ctx, b, update, "👎")
		return
	} else {
		setReaction(ctx, b, update, "👍")
	}
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
	case "":
		h.handleInlineMenu(ctx, b, update)
	case "np":
		h.handleNowPlaying(ctx, b, update)
	}
}

func (h *Handler) handleInlineMenu(
	ctx context.Context,
	b *bot.Bot,
	update *models.Update,
) {
	_, err := b.AnswerInlineQuery(ctx, &bot.AnswerInlineQueryParams{
		InlineQueryID: update.InlineQuery.ID,
		Results: []models.InlineQueryResult{
			&models.InlineQueryResultArticle{
				ID:          "menu-now-playing",
				Title:       "🎧 Now playing",
				Description: "Показать текущий трек",
				InputMessageContent: &models.InputTextMessageContent{
					MessageText: "Используй: @mybot np",
				},
			},
		},
		CacheTime:  1,
		IsPersonal: true,
	})

	if err != nil {
		slog.Error("answer_inline_menu", "error", err)
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

	text := fmt.Sprintf(
		"Играет:\n🎧 %s — %s\n💿 %s\n⏱ %s / %s",
		track.Artist,
		track.Title,
		track.Album,
		position,
		duration,
	)

	description := fmt.Sprintf(
		"%s · %s / %s",
		track.Album,
		position,
		duration,
	)

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

func clearReaction(
	ctx context.Context,
	b *bot.Bot,
	update *models.Update,
) {
	_, err := b.SetMessageReaction(ctx, &bot.SetMessageReactionParams{
		ChatID:    update.Message.Chat.ID,
		MessageID: update.Message.ID,
		Reaction:  []models.ReactionType{},
	})
	if err != nil {
		slog.Error("clear_reaction", "error", err)
	}
}

func setReaction(ctx context.Context, b *bot.Bot, update *models.Update, emoji string) {
	clearReaction(ctx, b, update)
	_, err := b.SetMessageReaction(ctx, &bot.SetMessageReactionParams{
		ChatID:    update.Message.Chat.ID,
		MessageID: update.Message.ID,
		Reaction: []models.ReactionType{
			{
				Type: models.ReactionTypeTypeEmoji,
				ReactionTypeEmoji: &models.ReactionTypeEmoji{
					Emoji: emoji,
				},
			},
		},
	})
	if err != nil {
		slog.Error("set_reaction", "error", err)
	}
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
	_, err := b.AnswerInlineQuery(ctx, &bot.AnswerInlineQueryParams{
		InlineQueryID: inlineQueryID,
		Results: []models.InlineQueryResult{
			&models.InlineQueryResultArticle{
				ID:          id,
				Title:       title,
				Description: description,
				InputMessageContent: &models.InputTextMessageContent{
					MessageText: message,
				},
			},
		},
		CacheTime:  1,
		IsPersonal: true,
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
