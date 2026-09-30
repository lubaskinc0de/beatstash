package poller

import (
	"context"
	"errors"
	"log/slog"
	"maps"
	"slices"
	"time"

	"gorm.io/gorm"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/import_collection"
)

// FollowedBatch is an Import started from the chat whose summary the chat
// has not got yet. Only such Imports are summed up.
type FollowedBatch struct {
	BatchID uint  `gorm:"primaryKey;autoIncrement:false"`
	ChatID  int64 `gorm:"not null"`
	// ClaimedUntil is the lease of the instance summing it up now.
	ClaimedUntil *time.Time
}

func (FollowedBatch) TableName() string {
	return "telegram_followed_batches"
}

type FollowedBatches struct {
	DB *gorm.DB
}

func (f *FollowedBatches) Follow(ctx context.Context, batch FollowedBatch) error {
	return f.DB.WithContext(ctx).Create(&batch).Error
}

// summarizeImports sends each summary at least once, like answerIngests.
type summarizeImports struct {
	batches outbox
	chats   Chats
	results *import_collection.GetImportResults
}

func (s *summarizeImports) Run(ctx context.Context) error {
	var followed []FollowedBatch
	if err := s.batches.waiting(ctx, &followed); err != nil || len(followed) == 0 {
		return err
	}
	byChat := map[int64][]uint{}
	for _, f := range followed {
		byChat[f.ChatID] = append(byChat[f.ChatID], f.BatchID)
	}
	recipients, err := s.chats.Recipients(ctx, slices.Sorted(maps.Keys(byChat)))
	if err != nil {
		return err
	}

	results := map[uint]*import_collection.ImportResult{}
	var finished, lost []uint
	for chatID, ids := range byChat {
		found, err := s.results.Execute(recipients.As(ctx, chatID), ids)
		switch {
		case errors.Is(err, import_collection.ErrImportNotFound), errors.Is(err, common.ErrNotAuthenticated):
			slog.Error("get_import_results", "chat_id", chatID, "error", err)
			lost = append(lost, ids...)
			continue
		case err != nil:
			return err
		}
		for n := range found {
			if found[n].Finished {
				results[found[n].ID] = &found[n]
				finished = append(finished, found[n].ID)
			}
		}
	}
	if err := s.batches.forget(ctx, lost); err != nil || len(finished) == 0 {
		return err
	}

	var claimed []FollowedBatch
	if err := s.batches.claim(ctx, finished, &claimed); err != nil || len(claimed) == 0 {
		return err
	}
	ids := make([]uint, 0, len(claimed))
	for _, f := range claimed {
		if err := s.chats.SummarizeImport(recipients.As(ctx, f.ChatID), f.ChatID, results[f.BatchID]); err != nil {
			slog.Error("summarize_import", "batch_id", f.BatchID, "error", err)
			continue
		}
		ids = append(ids, f.BatchID)
	}
	return s.batches.told(ctx, ids, *claimed[0].ClaimedUntil)
}
