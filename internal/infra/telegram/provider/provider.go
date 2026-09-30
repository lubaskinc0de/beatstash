package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/providers"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/ingest"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/trackfile"
)

const Name = provider.ProviderTelegram

type File struct {
	ID        string             `json:"file_id"`
	UniqueID  string             `json:"-"`
	Kind      trackfile.FileKind `json:"kind"`
	Name      string             `json:"file_name,omitempty"`
	Format    library.Format     `json:"format"`
	Performer string             `json:"performer,omitempty"`
	Title     string             `json:"title,omitempty"`
}

// Ref keys the Track Ref by file_unique_id, which stays the same across
// forwards, and keeps the rest in the payload.
func Ref(file File) (provider.TrackRef, error) {
	payload, err := json.Marshal(file)
	if err != nil {
		return provider.TrackRef{}, err
	}
	return provider.TrackRef{Provider: Name, ID: file.UniqueID, Payload: string(payload)}, nil
}

// LocalFile is a file a local Bot API server downloaded for a Track Ref,
// to be removed on Release by whichever instance finishes the Ingest Job.
type LocalFile struct {
	RefID string `gorm:"primaryKey"`
	Path  string `gorm:"not null"`
}

func (LocalFile) TableName() string {
	return "telegram_local_files"
}

type Provider struct {
	Bot   *bot.Bot
	Files *trackfile.Files
	DB    *gorm.DB
}

func (p *Provider) Name() provider.ProviderName {
	return Name
}

func (p *Provider) Fetch(ctx context.Context, _ uint, ref provider.TrackRef) (*providers.FetchedAudio, error) {
	var file File
	if err := json.Unmarshal([]byte(ref.Payload), &file); err != nil {
		return nil, providers.Permanent(ingest.ReasonInternal, fmt.Errorf("decode payload: %w", err))
	}

	info, err := p.Bot.GetFile(ctx, &bot.GetFileParams{FileID: file.ID})
	if err != nil {
		return nil, fmt.Errorf("get file: %w", err)
	}

	body, err := p.open(ctx, ref, info.FilePath)
	if err != nil {
		return nil, err
	}

	return &providers.FetchedAudio{
		Body:     body,
		FileName: file.Name,
		Format:   file.Format,
		WeakHint: library.Metadata{Artist: file.Performer, Title: file.Title},
	}, nil
}

// Recognize: a Track Ref is the file's unique id, and the bot remembers the
// files it gave out.
func (p *Provider) Recognize(ctx context.Context, ref provider.TrackRef) ([]uint, error) {
	return p.Files.Recognize(ctx, ref.ID)
}

// Release removes the file before its row: a crash between leaves the row
// for the next Release.
func (p *Provider) Release(ctx context.Context, ref provider.TrackRef) error {
	var files []LocalFile
	if err := p.DB.WithContext(ctx).Where("ref_id = ?", ref.ID).Find(&files).Error; err != nil || len(files) == 0 {
		return err
	}
	if err := os.Remove(files[0].Path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return p.DB.WithContext(ctx).Delete(&files[0]).Error
}

// open reads the file a local Bot API server (--local) put on the shared
// volume, or downloads it from a regular one.
func (p *Provider) open(ctx context.Context, ref provider.TrackRef, filePath string) (io.ReadCloser, error) {
	if filepath.IsAbs(filePath) {
		file, err := os.Open(filePath) //nolint:gosec // G304: path comes from our storage
		if err != nil {
			return nil, fmt.Errorf("open local file: %w", err)
		}
		err = p.DB.WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).Create(&LocalFile{RefID: ref.ID, Path: filePath}).Error
		if err != nil {
			_ = file.Close()
			return nil, fmt.Errorf("remember local file: %w", err)
		}
		return file, nil
	}

	url := p.Bot.FileDownloadLink(&models.File{FilePath: filePath})
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download file: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("download file: %s", resp.Status)
	}
	return resp.Body, nil
}
