package bot

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"time"

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

type trackFiles map[uint]*store.File

// of returns nil if the Track has no Telegram file yet.
func (f trackFiles) of(track *library.Track) *store.File {
	if track == nil {
		return nil
	}
	return f[track.ID]
}

// filesOf skips nil Tracks; on error the Tracks go without files.
func (h *Handler) filesOf(ctx context.Context, tracks ...*library.Track) trackFiles {
	ids := make([]uint, 0, len(tracks))
	for _, track := range tracks {
		if track != nil {
			ids = append(ids, track.ID)
		}
	}
	files, err := h.Files.Of(ctx, ids)
	if err != nil {
		slog.Error("find_telegram_files", "error", err)
	}
	return files
}

// uploadWait is how often a request waiting for another's upload looks
// for the file.
const uploadWait = 100 * time.Millisecond

// fileFor uploads the Track to the chat unless it has a file already;
// uploaded tells which. While somebody else uploads it, fileFor waits for
// their file.
func (h *Handler) fileFor(ctx context.Context, chatID int64, track *library.Track, path string) (file *store.File, uploaded bool, err error) {
	for {
		file, err := h.storedFile(ctx, track)
		if err != nil || file != nil {
			return file, false, err
		}
		claimed, err := h.Files.Claim(ctx, track.ID)
		if err != nil {
			return nil, false, err
		}
		if claimed {
			break
		}
		select {
		case <-ctx.Done():
			return nil, false, ctx.Err()
		case <-time.After(uploadWait):
		}
	}
	defer func() {
		if err := h.Files.Release(context.WithoutCancel(ctx), track.ID); err != nil {
			slog.Error("release_upload", "track_id", track.ID, "error", err)
		}
	}()

	// The previous claimant may have finished between the look and the claim.
	if file, err := h.storedFile(ctx, track); err != nil || file != nil {
		return file, false, err
	}
	posted, err := h.Sender.Post(ctx, chatID, path, track)
	if err != nil {
		return nil, false, err
	}
	// The file serves the upload that made it even if it is not remembered.
	if err := h.Files.Remember(ctx, *posted); err != nil {
		slog.Error("remember_telegram_file", "track_id", track.ID, "error", err)
	}
	return posted, true, nil
}

func (h *Handler) storedFile(ctx context.Context, track *library.Track) (*store.File, error) {
	files, err := h.Files.Of(ctx, []uint{track.ID})
	if err != nil {
		return nil, err
	}
	return files[track.ID], nil
}
