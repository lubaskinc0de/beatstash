package bot

import (
	"context"
	"sync"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

// progressEdits edits at most once per interval (Telegram limits edits) and
// makes a skipped edit later: the next job may wait out a pause or a retry.
type progressEdits struct {
	mu      sync.Mutex
	batches map[uint]*batchEdits
}

type batchEdits struct {
	mu       sync.Mutex
	editedAt time.Time
	latest   *repositories.BatchProgress
	finished bool
}

func (p *progressEdits) of(batchID uint) *batchEdits {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.batches == nil {
		p.batches = map[uint]*batchEdits{}
	}
	e, ok := p.batches[batchID]
	if !ok {
		e = &batchEdits{}
		p.batches[batchID] = e
	}
	return e
}

func (p *progressEdits) forget(batchID uint) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.batches, batchID)
}

func (p *progressEdits) progress(
	ctx context.Context,
	batch *domain.IngestBatch,
	progress repositories.BatchProgress,
	interval time.Duration,
	edit func(repositories.BatchProgress),
) {
	e := p.of(batch.ID)
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.finished {
		return
	}

	wait := interval - time.Since(e.editedAt)
	if wait <= 0 {
		e.editedAt = time.Now()
		edit(progress)
		return
	}
	scheduled := e.latest != nil
	e.latest = &progress
	if scheduled {
		return
	}
	time.AfterFunc(wait, func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		if e.finished || e.latest == nil || ctx.Err() != nil {
			return
		}
		e.editedAt = time.Now()
		edit(*e.latest)
		e.latest = nil
	})
}

func (p *progressEdits) finished(batchID uint, summarize func()) {
	e := p.of(batchID)
	e.mu.Lock()
	e.finished = true
	e.latest = nil
	summarize()
	e.mu.Unlock()
	p.forget(batchID)
}
