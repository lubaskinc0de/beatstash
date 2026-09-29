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
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/i18n"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/store"
)

// Poller reads what became of the jobs the bot's messages started and
// answers those messages: the core keeps no address to answer to. Its
// interval also paces the answers.
type Poller struct {
	Bot *bot.Bot
	// Handler shares its stores, texts and windows with the Poller.
	Handler *Handler

	Jobs            *ingest_track.GetIngestJobs
	Batches         *ingest_track.GetIngestBatches
	InvalidAccounts *sync_collection.GetInvalidatedProviderAccounts
	AccountNotices  *store.AccountNotices

	MusicDir string

	Interval time.Duration

	wake chan struct{}

	mu sync.Mutex
	// unpostable are the Tracks the storage chat did not take, e.g. too big.
	unpostable []uint
	// rounds counts finished rounds for WaitIdle.
	rounds int
}

// Wake starts the next round now: something may have happened that the
// user should hear of.
func (p *Poller) Wake() {
	select {
	case p.wakeChan() <- struct{}{}:
	default:
	}
}

func (p *Poller) wakeChan() chan struct{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.wake == nil {
		p.wake = make(chan struct{}, 1)
	}
	return p.wake
}

func (p *Poller) Run(ctx context.Context) {
	ticker := time.NewTicker(p.Interval)
	defer ticker.Stop()
	wake := p.wakeChan()

	steps := []struct {
		name string
		run  func(context.Context) error
	}{
		{"answer_ingest_jobs", p.answerJobs},
		{"summarize_ingest_batches", p.summarizeBatches},
		{"show_imports", p.showImports},
		{"notice_invalid_provider_accounts", p.noticeInvalidAccounts},
		{"post_to_storage_chat", p.postToStorageChat},
	}
	for {
		for _, step := range steps {
			if err := step.run(ctx); err != nil && ctx.Err() == nil {
				slog.Error(step.name, "error", err)
			}
		}
		p.mu.Lock()
		p.rounds++
		p.mu.Unlock()

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-wake:
		}
	}
}

// WaitIdle blocks until nothing is left to answer, then one more whole
// round, so the windows show the end too.
func (p *Poller) WaitIdle(ctx context.Context) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	idleSince := -1
	for {
		idle, err := p.idle(ctx)
		if err != nil {
			return err
		}
		p.mu.Lock()
		rounds := p.rounds
		p.mu.Unlock()
		switch {
		case !idle:
			idleSince = -1
		case idleSince < 0:
			idleSince = rounds
			p.Wake()
		case rounds >= idleSince+2:
			return nil
		default:
			p.Wake()
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (p *Poller) idle(ctx context.Context) (bool, error) {
	jobs, err := p.Handler.JobMessages.All(ctx)
	if err != nil {
		return false, err
	}
	batches, err := p.Handler.Followed.All(ctx)
	if err != nil {
		return false, err
	}
	unposted, err := p.unposted(ctx, 1)
	return len(jobs)+len(batches)+len(unposted) == 0, err
}

// texts: the chat is private, so its id is the user's. A user without a
// saved language gets the default.
func (p *Poller) texts(ctx context.Context, chatID int64) i18n.Catalog {
	lang, _, err := p.Handler.Users.Language(ctx, chatID)
	if err != nil {
		slog.Error("read_language", "error", err)
	}
	return p.Handler.Texts.For(i18n.Language(lang))
}

// answerJobs forgets a message only once it is answered, so a restart
// answers the rest.
func (p *Poller) answerJobs(ctx context.Context) error {
	messages, err := p.Handler.JobMessages.All(ctx)
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
		if err := p.Handler.JobMessages.Forget(ctx, m.JobID); err != nil {
			return err
		}
	}
	return nil
}

func (p *Poller) answer(ctx context.Context, msg messageRef, job *ingest.IngestJob) {
	c := p.texts(ctx, msg.chatID)
	if job.Failed() {
		reject(ctx, p.Bot, msg, c.UploadFailed(job.FailureReason))
		return
	}

	setReaction(ctx, p.Bot, msg, "👍")
	switch job.Outcome {
	case library.StoredInInbox:
		replyTo(ctx, p.Bot, msg, c.StoredInInbox())
	case library.AlreadyExists:
		replyTo(ctx, p.Bot, msg, c.AlreadyExists())
	case library.Stored, library.Replaced:
		// The reaction alone reports success.
	}
}

// summarizeBatches sends each summary once: the batch is forgotten right after.
func (p *Poller) summarizeBatches(ctx context.Context) error {
	followed, err := p.Handler.Followed.All(ctx)
	if err != nil || len(followed) == 0 {
		return err
	}
	ids := make([]uint, 0, len(followed))
	for _, f := range followed {
		ids = append(ids, f.BatchID)
	}
	states, err := p.Batches.Execute(ctx, ids)
	if err != nil {
		return err
	}
	byID := make(map[uint]*ingest_track.BatchState, len(states))
	for n := range states {
		byID[states[n].Batch.ID] = &states[n]
	}

	for _, f := range followed {
		state, known := byID[f.BatchID]
		if known && !state.Batch.Finished() {
			continue
		}
		if known {
			sendText(ctx, p.Bot, f.ChatID, p.texts(ctx, f.ChatID).ImportSummary(state))
		}
		if err := p.Handler.Followed.Forget(ctx, f.BatchID); err != nil {
			return err
		}
	}
	return nil
}

func (p *Poller) showImports(ctx context.Context) error {
	windows, err := p.Handler.Windows.OnScreen(ctx, string(screenImports))
	if err != nil {
		return err
	}
	for _, window := range windows {
		userCtx := withTexts(asRecipient(ctx, window.ChatID), p.texts(ctx, window.ChatID))
		p.Handler.refreshImports(userCtx, p.Bot, window.ChatID)
	}
	return nil
}

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
			c := p.texts(ctx, chatID)
			sendKeyboard(ctx, p.Bot, chatID, c.TokenRejected(account.Provider), &models.InlineKeyboardMarkup{
				InlineKeyboard: [][]models.InlineKeyboardButton{{
					goButton(c.Reconnect(), place{screen: screenProvider, arg: string(account.Provider)}),
				}},
			})
		}
		if err := p.AccountNotices.Remember(ctx, notice); err != nil {
			return err
		}
	}
	return nil
}

// rememberFile makes the file the user sent the Track's, for inline mode.
func (p *Poller) rememberFile(ctx context.Context, m store.JobMessage, job *ingest.IngestJob) error {
	if !job.Done() || job.TrackID == nil || m.FileID == "" {
		return nil
	}
	return p.Handler.Files.Remember(ctx, store.File{TrackID: *job.TrackID, ID: m.FileID, UniqueID: m.FileUniqueID, Kind: m.FileKind})
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
		_, _, err := p.Handler.fileFor(ctx, p.Handler.StorageChatID, &track.Track, filepath.Join(p.MusicDir, track.Dir, track.Path))
		if err != nil {
			slog.Error("post_track_to_storage_chat", "track_id", track.ID, "error", err)
			p.mu.Lock()
			p.unpostable = append(p.unpostable, track.ID)
			p.mu.Unlock()
		}
	}
	return nil
}

func (p *Poller) unposted(ctx context.Context, limit int) ([]store.Unfiled, error) {
	if p.Handler.StorageChatID == 0 {
		return nil, nil
	}
	p.mu.Lock()
	except := slices.Clone(p.unpostable)
	p.mu.Unlock()
	return p.Handler.Files.Unfiled(ctx, except, limit)
}
