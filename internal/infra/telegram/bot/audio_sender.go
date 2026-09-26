package bot

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/store"
)

type AudioSender struct {
	Bot         *bot.Bot
	MaxPostSize int64
}

var ErrFileTooLarge = errors.New("file is over the Bot API size limit")

func (s *AudioSender) Send(ctx context.Context, chatID int64, file *store.File) error {
	var err error
	if file.Kind == store.FileDocument {
		_, err = s.Bot.SendDocument(ctx, &bot.SendDocumentParams{ChatID: chatID, Document: &models.InputFileString{Data: file.ID}})
	} else {
		_, err = s.Bot.SendAudio(ctx, &bot.SendAudioParams{ChatID: chatID, Audio: &models.InputFileString{Data: file.ID}})
	}
	return err
}

// Post uploads the Track's file from disk and keeps what Telegram made of
// it: it may take a format it cannot play as a document. ErrFileTooLarge if
// Telegram would not take it.
func (s *AudioSender) Post(ctx context.Context, chatID int64, path string, track *library.Track) (*store.File, error) {
	file, err := os.Open(path) //nolint:gosec // G304: path comes from our storage
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > s.MaxPostSize {
		return nil, ErrFileTooLarge
	}

	msg, err := s.Bot.SendAudio(ctx, &bot.SendAudioParams{
		ChatID:    chatID,
		Audio:     &models.InputFileUpload{Filename: postedName(track), Data: file},
		Performer: track.Artist,
		Title:     track.Title,
		Duration:  track.DurationMs / 1000,
	})
	switch {
	case err != nil:
		return nil, err
	case msg.Audio != nil:
		return &store.File{TrackID: track.ID, ID: msg.Audio.FileID, UniqueID: msg.Audio.FileUniqueID, Kind: store.FileAudio}, nil
	case msg.Document != nil:
		return &store.File{TrackID: track.ID, ID: msg.Document.FileID, UniqueID: msg.Document.FileUniqueID, Kind: store.FileDocument}, nil
	default:
		return nil, errors.New("telegram kept the file as neither audio nor a document")
	}
}

func postedName(track *library.Track) string {
	return track.Title + track.Format.Ext()
}

// fileOf returns nil if the Track has no Telegram file yet.
func (h *Handler) fileOf(ctx context.Context, track *library.Track) *store.File {
	if track == nil {
		return nil
	}
	file, err := h.Files.For(ctx, track.ID)
	if err != nil {
		slog.Error("find_telegram_file", "track_id", track.ID, "error", err)
	}
	return file
}
