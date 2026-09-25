package ingest

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type Workers struct {
	tx       application.TxManager
	queue    application.IngestQueue
	pipeline *Pipeline
	notifier application.IngestNotifier

	count        int
	retryDelays  []time.Duration
	pollInterval time.Duration

	wake chan struct{}
	// busy counts workers between claiming a job and notifying about it.
	busy atomic.Int64
}

func NewWorkers(
	tx application.TxManager,
	queue application.IngestQueue,
	pipeline *Pipeline,
	notifier application.IngestNotifier,
	count int,
	retryDelays []time.Duration,
	pollInterval time.Duration,
) *Workers {
	return &Workers{
		tx:           tx,
		queue:        queue,
		pipeline:     pipeline,
		notifier:     notifier,
		count:        count,
		retryDelays:  retryDelays,
		pollInterval: pollInterval,
		wake:         make(chan struct{}, 1),
	}
}

func (w *Workers) Wake() {
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

// Start runs the workers until ctx is done; the returned channel closes
// once all of them have stopped.
func (w *Workers) Start(ctx context.Context) <-chan struct{} {
	if err := w.pipeline.ClearScratch(); err != nil {
		slog.Error("clear_ingest_scratch", "error", err)
	}

	var wg sync.WaitGroup
	for range w.count {
		wg.Go(func() { w.loop(ctx) })
	}

	stopped := make(chan struct{})
	go func() {
		wg.Wait()
		close(stopped)
	}()
	return stopped
}

// WaitIdle blocks until no job is pending and no worker is still
// finishing one.
func (w *Workers) WaitIdle(ctx context.Context) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		unfinished, err := w.queue.CountUnfinished(ctx)
		if err != nil {
			return err
		}
		if unfinished == 0 && w.busy.Load() == 0 {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (w *Workers) loop(ctx context.Context) {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		for w.runOnce(ctx) {
		}

		select {
		case <-ctx.Done():
			return
		case <-w.wake:
		case <-ticker.C:
		}
	}
}

func (w *Workers) runOnce(ctx context.Context) (worked bool) {
	w.busy.Add(1)
	defer w.busy.Add(-1)

	if ctx.Err() != nil {
		return false
	}

	var files library.FileChanges
	a, err := w.try(ctx, &files)
	files.Settle(errors.Join(err, a.err))
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			slog.Error("ingest_job_commit_failed", "error", err)
		}
		return false
	}
	if a.job == nil {
		return false
	}

	w.finish(ctx, a)
	return true
}

// attempt is one run of a job through the Pipeline; err is why it failed.
type attempt struct {
	job    *domain.IngestJob
	result *Result
	err    error
}

// try claims a due job and processes it in one transaction. The claim's
// row lock keeps other workers off the job, and the savepoint around Process
// lets a failed attempt still record itself. The error is the transaction's.
func (w *Workers) try(ctx context.Context, files *library.FileChanges) (attempt, error) {
	var a attempt
	err := w.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		a.job, err = w.queue.ClaimNext(ctx)
		if err != nil || a.job == nil {
			return err
		}

		a.job.Attempts++
		a.err = w.tx.WithinTx(ctx, func(ctx context.Context) error {
			a.result, err = w.pipeline.Process(ctx, a.job, files)
			return err
		})
		if ctx.Err() != nil {
			return ctx.Err()
		}

		w.recordAttempt(a.job, a.err)
		return w.queue.Save(ctx, a.job)
	})
	return a, err
}

func (w *Workers) finish(ctx context.Context, a attempt) {
	if a.job.Status != domain.IngestJobPending {
		if err := w.pipeline.Release(ctx, a.job); err != nil {
			slog.Error("ingest_job_release_failed", "job_id", a.job.ID, "error", err)
		}
	}

	msg := application.MessageRef{ChatID: a.job.ChatID, MessageID: a.job.MessageID}
	switch a.job.Status {
	case domain.IngestJobDone:
		slog.Info("ingest_job_done", "job_id", a.job.ID, "outcome", a.result.Outcome, "path", a.result.Path)
		w.notifier.Ingested(ctx, msg, a.result.Outcome)
	case domain.IngestJobFailed:
		w.notifier.IngestFailed(ctx, msg, failureReason(a.err))
	}
}

func (w *Workers) recordAttempt(job *domain.IngestJob, procErr error) {
	if procErr == nil {
		job.Status = domain.IngestJobDone
		job.LastError = ""
		return
	}

	job.LastError = procErr.Error()

	var permanent *application.PermanentError
	final := errors.As(procErr, &permanent) || job.Attempts > len(w.retryDelays)

	slog.Error(
		"ingest_job_failed",
		"job_id", job.ID,
		"step", failedStep(procErr),
		"attempt", job.Attempts,
		"final", final,
		"error", procErr,
	)

	if final {
		job.Status = domain.IngestJobFailed
		return
	}
	job.RunAt = time.Now().Add(w.retryDelays[job.Attempts-1])
}

func failedStep(err error) step {
	var se *stepError
	if errors.As(err, &se) {
		return se.step
	}
	return "unknown"
}

func failureReason(err error) application.FailureReason {
	var permanent *application.PermanentError
	if errors.As(err, &permanent) {
		return permanent.Reason
	}
	if failedStep(err) == stepFetch {
		return application.ReasonFetchFailed
	}
	return application.ReasonInternal
}
