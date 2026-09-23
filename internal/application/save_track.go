package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/lubaskinc0de/navidrome-tg/internal/entities"
)

type TrackData struct {
	TelegramFileID       string
	TelegramFileUniqueID string

	FileName string
	MimeType string
	FileSize int64
	Duration int

	Title     string
	Performer string

	MessageId   int
	DownloadUrl string
}

type SaveTrack struct {
	Repo TrackRepository
	Tx   TxManager
}

func NewSaveTrack(repo TrackRepository, tx TxManager) *SaveTrack {
	return &SaveTrack{
		Repo: repo,
		Tx:   tx,
	}
}

func (i *SaveTrack) Execute(
	ctx context.Context,
	data TrackData,
	musicDir string,
) error {
	return i.Tx.WithinTx(ctx, func(ctx context.Context) error {
		user, ok := UserFromContext(ctx)
		if !ok {
			return ErrNotAuthenticated
		}

		existing, err := i.Repo.GetByUniqueId(ctx, data.TelegramFileUniqueID)
		if err != nil && !errors.Is(err, ErrTrackNotFound) {
			return err
		}
		if existing != nil {
			return nil
		}

		fileName := trackFileName(
			data.TelegramFileUniqueID,
			data.FileName,
		)
		path := filepath.Join(musicDir, fileName)

		track := entities.Track{
			TelegramFileID:       data.TelegramFileID,
			TelegramFileUniqueID: data.TelegramFileUniqueID,
			FileName:             data.FileName,
			MimeType:             data.MimeType,
			FileSize:             data.FileSize,
			Duration:             data.Duration,
			Title:                data.Title,
			Performer:            data.Performer,
			MessageId:            data.MessageId,
			UserID:               user.ID,
			SavedToPath:          path,
		}

		err = i.Repo.Save(ctx, &track)
		if err != nil {
			return err
		}
		downloadFile(ctx, data.DownloadUrl, path)

		slog.Info("track_saved", "file_unique_id", track.TelegramFileUniqueID, "uid", user.TelegramID)
		return nil
	})
}

func downloadFile(ctx context.Context, url, path string) error {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		url,
		nil,
	)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download_failed: %s, %d", url, resp.StatusCode)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	return err
}

func trackFileName(fileUniqueID, originalName string) string {
	sum := sha256.Sum256([]byte(fileUniqueID))
	name := hex.EncodeToString(sum[:])

	ext := filepath.Ext(originalName)

	return name + ext
}
