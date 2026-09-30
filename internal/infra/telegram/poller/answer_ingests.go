package poller

import (
	"context"
	"errors"
	"log/slog"
	"maps"
	"slices"
	"time"

	"gorm.io/gorm"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/add_track"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/ingest"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/trackfile"
)

// JobMessage is the message whose audio became the Ingest Job: it gets the
// job's answer, and its file becomes the Track's.
type JobMessage struct {
	JobID     uint  `gorm:"primaryKey;autoIncrement:false"`
	ChatID    int64 `gorm:"not null"`
	MessageID int   `gorm:"not null"`

	FileID       string             `gorm:"not null"`
	FileUniqueID string             `gorm:"not null"`
	FileKind     trackfile.FileKind `gorm:"not null"`

	// ClaimedUntil is the lease of the instance answering it now.
	ClaimedUntil *time.Time
}

func (JobMessage) TableName() string {
	return "telegram_job_messages"
}

type JobMessages struct {
	DB *gorm.DB
}

func (m *JobMessages) Remember(ctx context.Context, message JobMessage) error {
	return m.DB.WithContext(ctx).Create(&message).Error
}

// answerIngests answers each message at least once: a message whose answer
// failed keeps its lease, so the next try comes once it runs out.
type answerIngests struct {
	messages outbox
	chats    Chats
	results  *add_track.GetIngestResults
	files    *trackfile.Files
}

func (s *answerIngests) Run(ctx context.Context) error {
	var messages []JobMessage
	if err := s.messages.waiting(ctx, &messages); err != nil || len(messages) == 0 {
		return err
	}
	byChat := map[int64][]uint{}
	for _, m := range messages {
		byChat[m.ChatID] = append(byChat[m.ChatID], m.JobID)
	}
	recipients, err := s.chats.Recipients(ctx, slices.Sorted(maps.Keys(byChat)))
	if err != nil {
		return err
	}

	jobs := map[uint]*ingest.IngestJob{}
	usages := map[int64]library.Usage{}
	var finished, lost []uint
	for chatID, ids := range byChat {
		results, err := s.results.Execute(recipients.As(ctx, chatID), ids)
		switch {
		case errors.Is(err, add_track.ErrIngestJobNotFound), errors.Is(err, common.ErrNotAuthenticated):
			slog.Error("get_ingest_results", "chat_id", chatID, "error", err)
			lost = append(lost, ids...)
			continue
		case err != nil:
			return err
		}
		usages[chatID] = results.Usage
		for n := range results.Jobs {
			if job := &results.Jobs[n]; job.Finished() {
				jobs[job.ID] = job
				finished = append(finished, job.ID)
			}
		}
	}
	if err := s.messages.forget(ctx, lost); err != nil || len(finished) == 0 {
		return err
	}

	var claimed []JobMessage
	if err := s.messages.claim(ctx, finished, &claimed); err != nil || len(claimed) == 0 {
		return err
	}
	var files []trackfile.File
	ids := make([]uint, 0, len(claimed))
	for _, m := range claimed {
		job := jobs[m.JobID]
		if err := s.chats.AnswerIngest(recipients.As(ctx, m.ChatID), m, job, usages[m.ChatID]); err != nil {
			slog.Error("answer_ingest", "job_id", m.JobID, "error", err)
			continue
		}
		ids = append(ids, m.JobID)
		if job.Done() && job.TrackID != nil && m.FileID != "" {
			files = append(files, trackfile.File{TrackID: *job.TrackID, ID: m.FileID, UniqueID: m.FileUniqueID, Kind: m.FileKind})
		}
	}
	// The file the user sent becomes the Track's, for inline mode.
	if err := s.files.Remember(ctx, files...); err != nil {
		return err
	}
	return s.messages.told(ctx, ids, *claimed[0].ClaimedUntil)
}
