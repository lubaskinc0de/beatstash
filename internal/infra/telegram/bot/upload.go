package bot

import (
	"context"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/ingest"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
	tgprovider "github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/provider"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/store"
)

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
		reject(ctx, b, msg, texts(ctx).UploadFailed(ingest.ReasonUnsupportedFormat))
		return
	}
	file.Format = format

	ref, err := tgprovider.Ref(file)
	if err != nil {
		slog.Error("track_ref", "error", err)
		reject(ctx, b, msg, texts(ctx).UploadFailed(ingest.ReasonInternal))
		return
	}

	// 👀 goes first: the Poller may set 👍 as soon as the job is remembered.
	setReaction(ctx, b, msg, "👀")
	jobID, err := h.EnqueueIngest.Execute(ctx, ref)
	if err != nil {
		slog.Error("enqueue_ingest", "error", err)
		reject(ctx, b, msg, texts(ctx).UploadFailed(ingest.ReasonInternal))
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
