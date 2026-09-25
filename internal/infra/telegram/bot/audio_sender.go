package bot

import (
	"context"
	"errors"
	"os"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type AudioSender struct {
	Bot         *bot.Bot
	MaxPostSize int64
}

func (s *AudioSender) Send(ctx context.Context, chatID int64, file *domain.TelegramFile) error {
	var err error
	if file.Kind == domain.TelegramFileDocument {
		_, err = s.Bot.SendDocument(ctx, &bot.SendDocumentParams{ChatID: chatID, Document: &models.InputFileString{Data: file.ID}})
	} else {
		_, err = s.Bot.SendAudio(ctx, &bot.SendAudioParams{ChatID: chatID, Audio: &models.InputFileString{Data: file.ID}})
	}
	return err
}

// Post keeps what Telegram made of the file: it may take a format it
// cannot play as a document.
func (s *AudioSender) Post(ctx context.Context, chatID int64, path string, track *domain.Track) (*common.PostedFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > s.MaxPostSize {
		return nil, common.ErrFileTooLarge
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
		return &common.PostedFile{
			File:     domain.TelegramFile{ID: msg.Audio.FileID, Kind: domain.TelegramFileAudio},
			UniqueID: msg.Audio.FileUniqueID,
		}, nil
	case msg.Document != nil:
		return &common.PostedFile{
			File:     domain.TelegramFile{ID: msg.Document.FileID, Kind: domain.TelegramFileDocument},
			UniqueID: msg.Document.FileUniqueID,
		}, nil
	default:
		return nil, errors.New("telegram kept the file as neither audio nor a document")
	}
}

func postedName(track *domain.Track) string {
	return track.Title + track.Format.Ext()
}
