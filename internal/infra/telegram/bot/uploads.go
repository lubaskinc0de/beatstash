package bot

import (
	"context"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/beatstash/internal/application/add_track"
	"github.com/lubaskinc0de/beatstash/internal/domain/ingest"
	"github.com/lubaskinc0de/beatstash/internal/domain/library"
	"github.com/lubaskinc0de/beatstash/internal/infra/telegram/poller"
	tgprovider "github.com/lubaskinc0de/beatstash/internal/infra/telegram/provider"
	"github.com/lubaskinc0de/beatstash/internal/infra/telegram/trackfile"
)

type Uploads struct {
	Telegram      *Telegram
	EnqueueIngest *add_track.EnqueueIngest
	JobMessages   *poller.JobMessages
}

func (u *Uploads) handleAudio(
	ctx context.Context, _ *bot.Bot,
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
		logUnsent(u.Telegram.reject(ctx, msg, texts(ctx).UploadFailed(ingest.ReasonUnsupportedFormat)))
		return
	}
	file.Format = format

	ref, err := tgprovider.Ref(file)
	if err != nil {
		slog.Error("track_ref", "error", err)
		logUnsent(u.Telegram.reject(ctx, msg, texts(ctx).UploadFailed(ingest.ReasonInternal)))
		return
	}

	// 👀 goes first: the Poller may set 👍 as soon as the job is remembered.
	logUnsent(u.Telegram.setReaction(ctx, msg, "👀"))
	jobID, err := u.EnqueueIngest.Execute(ctx, ref)
	if err != nil {
		slog.Error("enqueue_ingest", "error", err)
		logUnsent(u.Telegram.reject(ctx, msg, texts(ctx).UploadFailed(ingest.ReasonInternal)))
		return
	}
	err = u.JobMessages.Remember(ctx, poller.JobMessage{
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
			Kind:      trackfile.FileAudio,
			Name:      msg.Audio.FileName,
			Performer: msg.Audio.Performer,
			Title:     msg.Audio.Title,
		}, msg.Audio.MimeType, true
	case msg.Document != nil:
		return tgprovider.File{
			ID:       msg.Document.FileID,
			UniqueID: msg.Document.FileUniqueID,
			Kind:     trackfile.FileDocument,
			Name:     msg.Document.FileName,
		}, msg.Document.MimeType, true
	}
	return tgprovider.File{}, "", false
}
