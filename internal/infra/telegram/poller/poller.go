// Package poller tells the chats what became of what they started: the
// core keeps no address to answer to. What it told already it keeps in
// tables of its own, so any instance may tell it, and only one does.
package poller

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"gorm.io/gorm"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/add_track"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/connect_provider"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/import_collection"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/ingest"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/trackfile"
)

// Chats is the bot's side of the Poller: it knows the chats' users and
// what to tell them.
type Chats interface {
	Recipients(ctx context.Context, chatIDs []int64) (Recipients, error)
	// ChatsOf leaves out the Users who never used Telegram.
	ChatsOf(ctx context.Context, userIDs []uint) (map[uint]int64, error)
	// AnswerIngest and SummarizeImport fail only if a later try may succeed.
	AnswerIngest(ctx context.Context, msg JobMessage, job *ingest.IngestJob) error
	SummarizeImport(ctx context.Context, chatID int64, result *import_collection.ImportResult) error
	NoticeRejectedToken(ctx context.Context, chatID int64, providerName provider.ProviderName)
	// RefreshImports redraws the windows on the Imports.
	RefreshImports(ctx context.Context) error
}

// Recipients act as the users of the chats, in their languages; a chat is
// private, so its id is the user's.
type Recipients interface {
	As(ctx context.Context, chatID int64) context.Context
}

type Config struct {
	DB    *gorm.DB
	Chats Chats
	Files *trackfile.Files

	IngestResults  *add_track.GetIngestResults
	ImportResults  *import_collection.GetImportResults
	RejectedTokens *connect_provider.ListRejectedTokens

	// StorageChatID gets the Tracks without a Telegram file; zero turns it off.
	StorageChatID int64
	// FillStorageChat uploads them there in the background.
	FillStorageChat bool
	MusicDir        string

	// LeaseTTL outlasts an answer: an older claim on one was left by an
	// instance that died amid it.
	LeaseTTL time.Duration
	// Interval also paces the answers.
	Interval time.Duration
}

type Poller struct {
	db       *gorm.DB
	files    *trackfile.Files
	fill     bool
	interval time.Duration
	steps    []namedStep

	wake chan struct{}
	// rounds counts finished rounds for WaitIdle.
	rounds atomic.Int64
}

type namedStep struct {
	name string
	run  func(ctx context.Context) error
}

func New(c Config) *Poller {
	p := &Poller{
		db: c.DB, files: c.Files, fill: c.StorageChatID != 0 && c.FillStorageChat, interval: c.Interval,
		wake: make(chan struct{}, 1),
	}
	p.steps = []namedStep{
		{"answer_ingests", (&answerIngests{
			messages: outbox{db: c.DB, table: "telegram_job_messages", key: "job_id", leaseTTL: c.LeaseTTL},
			chats:    c.Chats, results: c.IngestResults, files: c.Files,
		}).Run},
		{"summarize_imports", (&summarizeImports{
			batches: outbox{db: c.DB, table: "telegram_followed_batches", key: "batch_id", leaseTTL: c.LeaseTTL},
			chats:   c.Chats, results: c.ImportResults,
		}).Run},
		{"refresh_imports", c.Chats.RefreshImports},
		{"notice_rejected_tokens", (&noticeRejectedTokens{db: c.DB, chats: c.Chats, tokens: c.RejectedTokens}).Run},
	}
	if p.fill {
		p.steps = append(p.steps, namedStep{"fill_storage_chat", (&fillStorageChat{
			files: c.Files, chatID: c.StorageChatID, musicDir: c.MusicDir,
		}).Run})
	}
	return p
}

// Wake starts the next round now: something may have happened that the
// user should hear of.
func (p *Poller) Wake() {
	select {
	case p.wake <- struct{}{}:
	default:
	}
}

func (p *Poller) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		for _, s := range p.steps {
			if err := s.run(ctx); err != nil && ctx.Err() == nil {
				slog.Error(s.name, "error", err)
			}
		}
		p.rounds.Add(1)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-p.wake:
		}
	}
}

// WaitIdle blocks until nothing is left to tell, then one more whole
// round, so the windows show the end too.
func (p *Poller) WaitIdle(ctx context.Context) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	idleSince := int64(-1)
	for {
		idle, err := p.idle(ctx)
		if err != nil {
			return err
		}
		rounds := p.rounds.Load()
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
	var waiting bool
	err := p.db.WithContext(ctx).Raw(`SELECT
		EXISTS (SELECT 1 FROM telegram_job_messages) OR EXISTS (SELECT 1 FROM telegram_followed_batches)`,
	).Scan(&waiting).Error
	if err != nil || waiting || !p.fill {
		return !waiting, err
	}
	unfiled, err := p.files.Unfiled(ctx, 1)
	return len(unfiled) == 0, err
}
