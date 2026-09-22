package telegram

import (
	"context"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/lubaskinc0de/navidrome-tg/internal/application"
	"github.com/lubaskinc0de/navidrome-tg/internal/config"
	"github.com/lubaskinc0de/navidrome-tg/internal/database"
)

type Handler struct {
	saveTrack *application.SaveTrack
	users     *database.UserRepository
	config    config.Config
}

func NewHandler(
	saveTrack *application.SaveTrack,
	users *database.UserRepository,
	config config.Config,
) *Handler {
	return &Handler{
		saveTrack: saveTrack,
		users:     users,
		config:    config,
	}
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
