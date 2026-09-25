package common

import (
	"context"
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type MessageRef struct {
	ChatID    int64
	MessageID int
}

type AudioSender interface {
	Send(ctx context.Context, chatID int64, file *domain.TelegramFile) error
	// Post uploads the file from disk; ErrFileTooLarge if Telegram would not take it.
	Post(ctx context.Context, chatID int64, path string, track *domain.Track) (*PostedFile, error)
}

type PostedFile struct {
	File     domain.TelegramFile
	UniqueID string
}

var ErrFileTooLarge = errors.New("file is over the Bot API size limit")

type BatchReporter interface {
	Announce(ctx context.Context, batch *domain.IngestBatch) (int, error)
	Progress(ctx context.Context, batch *domain.IngestBatch, progress repositories.BatchProgress)
	// Withdraw takes back the progress message of a batch that did not
	// start: the caller tells the user why.
	Withdraw(ctx context.Context, batch *domain.IngestBatch)
	Finished(ctx context.Context, batch *domain.IngestBatch, progress repositories.BatchProgress, failed []string)
}
