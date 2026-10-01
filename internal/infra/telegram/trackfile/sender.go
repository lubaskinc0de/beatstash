package trackfile

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

type AudioSender struct {
	Bot         *bot.Bot
	MaxPostSize int64
}

var ErrFileTooLarge = errors.New("file is over the Bot API size limit")

// TooLargeError is ErrFileTooLarge with the size of the file.
type TooLargeError struct {
	Size int64
}

func (e *TooLargeError) Error() string {
	return fmt.Sprintf("%s: %d bytes", ErrFileTooLarge, e.Size)
}

func (e *TooLargeError) Is(target error) bool {
	return target == ErrFileTooLarge
}

func (s *AudioSender) Send(ctx context.Context, chatID int64, file *File) error {
	var err error
	if file.Kind == FileDocument {
		_, err = s.Bot.SendDocument(ctx, &bot.SendDocumentParams{ChatID: chatID, Document: &models.InputFileString{Data: file.ID}})
	} else {
		_, err = s.Bot.SendAudio(ctx, &bot.SendAudioParams{ChatID: chatID, Audio: &models.InputFileString{Data: file.ID}})
	}
	return err
}

// post uploads the Track's file from disk and keeps what Telegram made of
// it: it may take a format it cannot play as a document. A *TooLargeError
// if Telegram would not take it.
func (s *AudioSender) post(ctx context.Context, chatID int64, path string, track *library.Track) (*File, error) {
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
		return nil, &TooLargeError{Size: info.Size()}
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
		return &File{TrackID: track.ID, ID: msg.Audio.FileID, UniqueID: msg.Audio.FileUniqueID, Kind: FileAudio, FileVersion: track.FileVersion}, nil
	case msg.Document != nil:
		return &File{
			TrackID: track.ID, ID: msg.Document.FileID, UniqueID: msg.Document.FileUniqueID, Kind: FileDocument, FileVersion: track.FileVersion,
		}, nil
	default:
		return nil, errors.New("telegram kept the file as neither audio nor a document")
	}
}

func postedName(track *library.Track) string {
	return track.Title + track.Format.Ext()
}
