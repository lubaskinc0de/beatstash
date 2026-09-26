package bot

import (
	"context"
	"log/slog"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/ingest_track"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/sync_collection"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/ingest"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/store"
)

var failureTexts = map[ingest.FailureReason]string{
	ingest.ReasonUnsupportedFormat: "Трек не загружен: формат не поддерживается. Подходят mp3, flac, m4a, ogg, opus и wav",
	ingest.ReasonCorruptFile:       "Трек не загружен: файл повреждён или это не аудио",
	ingest.ReasonFetchFailed:       "Трек не загружен: Telegram не отдал файл, пришлите его ещё раз",
	ingest.ReasonInternal:          "Трек не загружен: внутренняя ошибка, попробуйте позже",
}

// Poller reads what became of the jobs the bot's messages started and
// answers those messages: the core keeps no address to answer to. Its
// interval also paces the answers.
type Poller struct {
	Bot         *bot.Bot
	Jobs        *ingest_track.GetIngestJobs
	JobMessages *store.JobMessages

	Batches       *ingest_track.GetIngestBatches
	BatchMessages *store.BatchMessages

	InvalidAccounts *sync_collection.GetInvalidatedProviderAccounts
	AccountNotices  *store.AccountNotices

	Files  *store.Files
	Sender *AudioSender
	// StorageChatID gets the Tracks without a Telegram file, so inline mode
	// sends them as audio; zero turns it off.
	StorageChatID int64
	MusicDir      string

	Interval time.Duration

	mu sync.Mutex
	// unpostable are the Tracks the storage chat did not take, e.g. too big.
	unpostable []uint
}

func (p *Poller) Run(ctx context.Context) {
	ticker := time.NewTicker(p.Interval)
	defer ticker.Stop()

	for {
		if err := p.answerJobs(ctx); err != nil && ctx.Err() == nil {
			slog.Error("answer_ingest_jobs", "error", err)
		}
		if err := p.showBatches(ctx); err != nil && ctx.Err() == nil {
			slog.Error("show_ingest_batches", "error", err)
		}
		if err := p.noticeInvalidAccounts(ctx); err != nil && ctx.Err() == nil {
			slog.Error("notice_invalid_provider_accounts", "error", err)
		}
		if err := p.postToStorageChat(ctx); err != nil && ctx.Err() == nil {
			slog.Error("post_to_storage_chat", "error", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// WaitIdle blocks until every message the Poller follows has its answer.
func (p *Poller) WaitIdle(ctx context.Context) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		jobs, err := p.JobMessages.All(ctx)
		if err != nil {
			return err
		}
		batches, err := p.BatchMessages.All(ctx)
		if err != nil {
			return err
		}
		unposted, err := p.unposted(ctx, 1)
		if err != nil || len(jobs)+len(batches)+len(unposted) == 0 {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// answerJobs forgets a message only once it is answered, so a restart
// answers the rest.
func (p *Poller) answerJobs(ctx context.Context) error {
	messages, err := p.JobMessages.All(ctx)
	if err != nil || len(messages) == 0 {
		return err
	}
	ids := make([]uint, 0, len(messages))
	for _, m := range messages {
		ids = append(ids, m.JobID)
	}
	jobs, err := p.Jobs.Execute(ctx, ids)
	if err != nil {
		return err
	}
	byID := make(map[uint]*ingest.IngestJob, len(jobs))
	for n := range jobs {
		byID[jobs[n].ID] = &jobs[n]
	}

	for _, m := range messages {
		job, known := byID[m.JobID]
		if known && !job.Finished() {
			continue
		}
		if known {
			p.answer(ctx, messageRef{chatID: m.ChatID, messageID: m.MessageID}, job)
			if err := p.rememberFile(ctx, m, job); err != nil {
				return err
			}
		}
		if err := p.JobMessages.Forget(ctx, m.JobID); err != nil {
			return err
		}
	}
	return nil
}

func (p *Poller) answer(ctx context.Context, msg messageRef, job *ingest.IngestJob) {
	if job.Status == ingest.IngestJobFailed {
		reject(ctx, p.Bot, msg, failureTexts[job.FailureReason])
		return
	}

	setReaction(ctx, p.Bot, msg, "👍")
	switch job.Outcome {
	case library.StoredInInbox:
		replyTo(ctx, p.Bot, msg, "Трек попал в Inbox: не удалось определить исполнителя или название")
	case library.AlreadyExists:
		replyTo(ctx, p.Bot, msg, "Этот трек уже есть в библиотеке")
	}
}

// showBatches edits a progress message only when its text changes; the
// summary ends it.
func (p *Poller) showBatches(ctx context.Context) error {
	messages, err := p.BatchMessages.All(ctx)
	if err != nil || len(messages) == 0 {
		return err
	}
	ids := make([]uint, 0, len(messages))
	for _, m := range messages {
		ids = append(ids, m.BatchID)
	}
	states, err := p.Batches.Execute(ctx, ids)
	if err != nil {
		return err
	}
	byID := make(map[uint]*ingest_track.BatchState, len(states))
	for n := range states {
		byID[states[n].Batch.ID] = &states[n]
	}

	for _, m := range messages {
		state, known := byID[m.BatchID]
		if !known {
			if err := p.BatchMessages.Forget(ctx, m.BatchID); err != nil {
				return err
			}
			continue
		}
		if text := batchText(state); text != m.Shown {
			p.edit(ctx, m, text)
			if err := p.BatchMessages.Shown(ctx, m.BatchID, text); err != nil {
				return err
			}
		}
		if state.Finished() {
			if err := p.BatchMessages.Forget(ctx, m.BatchID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *Poller) edit(ctx context.Context, m store.BatchMessage, text string) {
	_, err := p.Bot.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    m.ChatID,
		MessageID: m.MessageID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	})
	if err != nil {
		slog.Error("edit_progress_message", "batch_id", m.BatchID, "error", err)
	}
}

const tokenRejected = "🔌 Звук перестал принимать токен, поэтому новые лайки и треки плейлистов больше не подтягиваются. " +
	"Подключите Звук заново: <code>/zvuk токен</code>"

// invalidationLag: an invalidation is stamped before its transaction
// commits, so it may show up after a later-stamped one was noticed already.
const invalidationLag = time.Minute

// noticeInvalidAccounts tells a user once per invalidation.
func (p *Poller) noticeInvalidAccounts(ctx context.Context) error {
	latest, err := p.AccountNotices.Latest(ctx)
	if err != nil {
		return err
	}
	accounts, err := p.InvalidAccounts.Execute(ctx, latest.Add(-invalidationLag))
	if err != nil {
		return err
	}
	for _, account := range accounts {
		notice := store.AccountNotice{UserID: account.UserID, Provider: string(account.Provider), InvalidatedAt: *account.InvalidatedAt}
		heard, err := p.AccountNotices.Heard(ctx, notice)
		if err != nil {
			return err
		}
		if heard {
			continue
		}
		chatID, ok, err := p.AccountNotices.ChatOf(ctx, account.UserID)
		if err != nil {
			return err
		}
		if ok {
			sendText(ctx, p.Bot, chatID, tokenRejected)
		}
		if err := p.AccountNotices.Remember(ctx, notice); err != nil {
			return err
		}
	}
	return nil
}

// rememberFile makes the file the user sent the Track's, for inline mode.
func (p *Poller) rememberFile(ctx context.Context, m store.JobMessage, job *ingest.IngestJob) error {
	if job.Status != ingest.IngestJobDone || job.TrackID == nil || m.FileID == "" {
		return nil
	}
	return p.Files.Remember(ctx, store.File{TrackID: *job.TrackID, ID: m.FileID, UniqueID: m.FileUniqueID, Kind: m.FileKind})
}

const storageBatch = 10

// postToStorageChat works on the adapter's own tables: whether a Track has
// a Telegram file is no question for the core.
func (p *Poller) postToStorageChat(ctx context.Context) error {
	tracks, err := p.unposted(ctx, storageBatch)
	if err != nil {
		return err
	}
	for n := range tracks {
		track := &tracks[n]
		posted, err := p.Sender.Post(ctx, p.StorageChatID, filepath.Join(p.MusicDir, track.Dir, track.Path), &track.Track)
		if err != nil {
			slog.Error("post_track_to_storage_chat", "track_id", track.ID, "error", err)
			p.mu.Lock()
			p.unpostable = append(p.unpostable, track.ID)
			p.mu.Unlock()
			continue
		}
		if err := p.Files.Remember(ctx, *posted); err != nil {
			return err
		}
	}
	return nil
}

func (p *Poller) unposted(ctx context.Context, limit int) ([]store.Unfiled, error) {
	if p.StorageChatID == 0 {
		return nil, nil
	}
	p.mu.Lock()
	except := slices.Clone(p.unpostable)
	p.mu.Unlock()
	return p.Files.Unfiled(ctx, except, limit)
}
