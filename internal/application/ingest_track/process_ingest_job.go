package ingest_track

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/providers"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type ProcessIngestJob struct {
	Tx        repositories.TxManager
	Queue     repositories.IngestQueue
	Providers *providers.Registry
	Tracks    repositories.Tracks
	Uploads   UploadRepository
	Libraries repositories.Libraries
	Lock      repositories.LibraryLock
	Disk      common.Disk
	Tags      AudioTags
	Remuxer   Remuxer
	Notifier  IngestNotifier
	Batches   repositories.IngestBatches
	Reporter  common.BatchReporter
	Sender    common.AudioSender
	InFlight  *InFlight
	MusicDir  string
	// StorageChatID gets new Tracks without a Telegram file, for inline
	// mode; zero turns it off.
	StorageChatID int64
	RetryDelays   []time.Duration
}

type IngestNotifier interface {
	Ingested(ctx context.Context, msg common.MessageRef, outcome IngestOutcome)
	IngestFailed(ctx context.Context, msg common.MessageRef, reason providers.FailureReason)
}

type IngestOutcome int

const (
	IngestStored IngestOutcome = iota
	IngestStoredInInbox
	IngestAlreadyExists
	IngestReplaced
)

type UploadRepository interface {
	Save(ctx context.Context, upload *domain.Upload) error
}

type AudioTags interface {
	// Probe returns ErrCorruptAudio for unreadable tags or no duration.
	Probe(path string) (*AudioProbe, error)
	WriteTags(path string, m domain.Metadata) error
	WriteCover(path string, image []byte) error
}

type AudioProbe struct {
	Tags       domain.Metadata
	DurationMs int
	Quality    domain.Quality
}

type Remuxer interface {
	// Codec returns "" if it cannot tell.
	Codec(ctx context.Context, path string) string
	WavToFlac(ctx context.Context, wav string) (string, error)
	// UnpackFlac takes FLAC out of MP4 without re-encoding.
	UnpackFlac(ctx context.Context, mp4 string) (string, error)
}

var ErrCorruptAudio = errors.New("audio file is corrupt")

// InFlight counts jobs from claim to notification; empty polls don't
// count, or WaitIdle would rarely see zero.
type InFlight struct {
	n atomic.Int64
}

func (f *InFlight) start() { f.n.Add(1) }

func (f *InFlight) done() { f.n.Add(-1) }

func (f *InFlight) Idle() bool {
	return f.n.Load() == 0
}

// Execute returns false if no job was processed.
func (i *ProcessIngestJob) Execute(ctx context.Context, filter repositories.JobFilter) (worked bool) {
	if ctx.Err() != nil {
		return false
	}

	var files libraries.FileChanges
	a, err := i.try(ctx, filter, &files)
	if a.job != nil {
		defer i.InFlight.done()
	}
	files.Settle(errors.Join(err, a.err))
	if err != nil {
		if ctx.Err() == nil {
			slog.Error("ingest_job_commit_failed", "error", err)
		}
		return false
	}
	if a.job == nil {
		return false
	}

	i.finish(ctx, a)
	return true
}

// attempt is one run of a job; err is why it failed.
type attempt struct {
	job    *domain.IngestJob
	result *jobResult
	err    error
}

// try claims a due job and processes it in one transaction. The claim's
// row lock keeps other workers off the job, and the savepoint around Process
// lets a failed attempt still record itself. The error is the transaction's.
func (i *ProcessIngestJob) try(ctx context.Context, filter repositories.JobFilter, files *libraries.FileChanges) (attempt, error) {
	var a attempt
	err := i.Tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		a.job, err = i.Queue.ClaimNext(ctx, filter)
		if err != nil || a.job == nil {
			return err
		}
		i.InFlight.start()

		a.job.Attempts++
		a.err = i.Tx.WithinTx(ctx, func(ctx context.Context) error {
			a.result, err = i.process(ctx, a.job, files)
			return err
		})
		if ctx.Err() != nil {
			return ctx.Err()
		}

		i.recordAttempt(a.job, a.err)
		return i.Queue.Save(ctx, a.job)
	})
	return a, err
}

func (i *ProcessIngestJob) finish(ctx context.Context, a attempt) {
	if a.job.Status != domain.IngestJobPending {
		if err := i.release(ctx, a.job); err != nil {
			slog.Error("ingest_job_release_failed", "job_id", a.job.ID, "error", err)
		}
	}

	if a.job.Status == domain.IngestJobDone {
		slog.Info("ingest_job_done", "job_id", a.job.ID, "outcome", a.result.Outcome, "path", a.result.Path)
		if a.result.TrackID != 0 && i.StorageChatID != 0 {
			if err := i.postToStorageChat(ctx, a.result.TrackID); err != nil {
				slog.Error("post_to_storage_chat", "track_id", a.result.TrackID, "error", err)
			}
		}
	}

	if a.job.Status == domain.IngestJobPending {
		return
	}
	if a.job.BatchID != nil {
		if err := reportBatch(ctx, i.Tx, i.Queue, i.Batches, i.Reporter, *a.job.BatchID); err != nil {
			slog.Error("report_ingest_batch", "batch_id", *a.job.BatchID, "error", err)
		}
		return
	}
	if a.job.ChatID == 0 {
		return
	}
	if a.job.Status == domain.IngestJobDone {
		i.Notifier.Ingested(ctx, messageOf(a.job), a.result.Outcome)
		return
	}
	i.Notifier.IngestFailed(ctx, messageOf(a.job), failureReason(a.err))
}

func messageOf(job *domain.IngestJob) common.MessageRef {
	return common.MessageRef{ChatID: job.ChatID, MessageID: job.MessageID}
}

func (i *ProcessIngestJob) recordAttempt(job *domain.IngestJob, procErr error) {
	if procErr == nil {
		job.Status = domain.IngestJobDone
		job.LastError = ""
		return
	}

	job.LastError = procErr.Error()

	var permanent *providers.PermanentError
	final := errors.As(procErr, &permanent) || job.Attempts > len(i.RetryDelays)

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
	job.RunAt = time.Now().Add(i.RetryDelays[job.Attempts-1])
}

func failedStep(err error) step {
	var se *stepError
	if errors.As(err, &se) {
		return se.step
	}
	return "unknown"
}

func failureReason(err error) providers.FailureReason {
	var permanent *providers.PermanentError
	if errors.As(err, &permanent) {
		return permanent.Reason
	}
	if failedStep(err) == stepFetch {
		return providers.ReasonFetchFailed
	}
	return providers.ReasonInternal
}

func (i *ProcessIngestJob) postToStorageChat(ctx context.Context, trackID uint) error {
	_, err := i.Tracks.TelegramFile(ctx, trackID)
	if !errors.Is(err, repositories.ErrNoTelegramFile) {
		return err
	}
	track, err := i.Tracks.Get(ctx, trackID)
	if err != nil {
		return err
	}
	path, err := libraries.TrackPath(ctx, i.Libraries, i.MusicDir, track)
	if err != nil {
		return err
	}
	posted, err := i.Sender.Post(ctx, i.StorageChatID, path, track)
	if err != nil {
		return err
	}
	return i.Tx.WithinTx(ctx, func(ctx context.Context) error {
		return libraries.RecordPosted(ctx, i.Tracks, track, posted)
	})
}

type jobResult struct {
	Outcome IngestOutcome
	Path    string
	TrackID uint
}

func alreadyExists() *jobResult {
	return &jobResult{Outcome: IngestAlreadyExists}
}

// incoming carries what process learned about the audio down to the store step.
type incoming struct {
	job     *domain.IngestJob
	library *domain.Library
	dir     string
	ref     domain.TrackRef
	audio   *providers.FetchedAudio
	format  domain.Format
	probe   *AudioProbe
	meta    domain.Metadata
	staged  string
	files   *libraries.FileChanges
}

// process turns the job's audio into a Track. Database writes go through
// ctx's transaction; the file is moved into the Library last, and files
// records how to make the Library follow the transaction's outcome.
func (i *ProcessIngestJob) process(ctx context.Context, job *domain.IngestJob, files *libraries.FileChanges) (*jobResult, error) {
	ref := job.Ref()

	personal, err := i.Libraries.Personal(ctx, job.UserID)
	if err != nil {
		return nil, wrapStep(stepSource, err)
	}
	shared, err := i.Libraries.Shared(ctx)
	if err != nil {
		return nil, wrapStep(stepSource, err)
	}
	libs := libraries.UserLibraries{Personal: personal, Shared: shared}

	// A forwarded copy of a known file needs no download at all, and
	// neither does the very file somebody shared.
	if result, err := i.knownSource(ctx, job, personal, ref); result != nil || err != nil {
		return result, wrapStep(stepSource, err)
	}
	inShared, err := i.inLibrary(ctx, shared, ref)
	if err != nil {
		return nil, wrapStep(stepSource, err)
	}
	if inShared {
		if result, err := i.lockAndRecheck(ctx, job, libs, ref, files); result != nil || err != nil {
			return result, wrapStep(stepSource, err)
		}
	}

	// A Provider without the Capability will not grow it on retry.
	fetcher, err := i.Providers.Fetcher(ref.Provider)
	if err != nil {
		return nil, wrapStep(stepFetch, providers.Permanent(providers.ReasonInternal, err))
	}

	audio, err := fetcher.Fetch(ctx, job.UserID, ref)
	if errors.Is(err, repositories.ErrProviderAccountNotFound) {
		return nil, wrapStep(stepFetch, providers.Permanent(providers.ReasonNoProviderAccount, err))
	}
	if errors.Is(err, providers.ErrUnauthorized) {
		return nil, wrapStep(stepFetch, providers.Permanent(providers.ReasonTokenRejected, err))
	}
	if err != nil {
		return nil, wrapStep(stepFetch, err)
	}

	// From here on the pipeline works on its own copy; the deferred removal
	// is a no-op once the file has been moved into the Library.
	staged, err := i.stage(audio)
	if err != nil {
		return nil, wrapStep(stepStage, err)
	}
	defer i.Disk.Remove(staged)

	// WAV carries no proper tags and taglib sees nothing of FLAC packed
	// into MP4, so both become FLAC files first.
	format := audio.Format
	var remux func(context.Context, string) (string, error)
	switch {
	case format == domain.FormatWAV:
		remux = i.Remuxer.WavToFlac
	case format == domain.FormatM4A && i.Remuxer.Codec(ctx, staged) == "flac":
		remux = i.Remuxer.UnpackFlac
	}
	if remux != nil {
		if staged, err = remux(ctx, staged); err != nil {
			return nil, wrapStep(stepRemux, err)
		}
		defer i.Disk.Remove(staged)
		format = domain.FormatFLAC
	}

	probe, err := i.Tags.Probe(staged)
	if errors.Is(err, ErrCorruptAudio) {
		return nil, wrapStep(stepProbe, providers.Permanent(providers.ReasonCorruptFile, err))
	}
	if err != nil {
		return nil, wrapStep(stepProbe, err)
	}

	// Each field comes from the first source that has it.
	meta := audio.Hint.Merge(
		probe.Tags,
		audio.WeakHint,
		domain.MetadataFromFileName(audio.FileName),
	).Normalize()

	// Workers download and probe in parallel, but the duplicate check and
	// path choice run one at a time per Library: otherwise two workers could
	// both miss the duplicate and store the same track twice. Another job may
	// also have added this source while we were fetching.
	if result, err := i.lockAndRecheck(ctx, job, libs, ref, files); result != nil || err != nil {
		return result, wrapStep(stepSource, err)
	}

	in := incoming{
		job:     job,
		library: personal,
		dir:     libraries.Dir(i.MusicDir, personal),
		ref:     ref,
		audio:   audio,
		format:  format,
		probe:   probe,
		meta:    meta,
		staged:  staged,
		files:   files,
	}

	// Inbox tracks lack the fields a Duplicate is matched by.
	if meta.Complete() {
		duplicate, err := i.Tracks.FindDuplicate(ctx, personal.ID, meta, probe.DurationMs)
		switch {
		case err == nil:
			result, err := i.mergeDuplicate(ctx, in, duplicate)
			return result, wrapStep(stepStore, err)
		case !errors.Is(err, repositories.ErrTrackNotFound):
			return nil, wrapStep(stepStore, err)
		}
	}

	result, err := i.storeNew(ctx, in)
	return result, wrapStep(stepStore, err)
}

// release is called once the job is finished for good; Providers that keep
// nothing between attempts simply lack the Capability.
func (i *ProcessIngestJob) release(ctx context.Context, job *domain.IngestJob) error {
	releaser, err := i.Providers.Releaser(job.Provider)
	if errors.Is(err, providers.ErrCapabilityNotSupported) {
		return nil
	}
	if err != nil {
		return err
	}
	return releaser.Release(ctx, job.Ref())
}

// knownSource records the Upload for a Track Ref already stored in the
// library, or returns nil when the ref is new there.
func (i *ProcessIngestJob) knownSource(
	ctx context.Context,
	job *domain.IngestJob,
	lib *domain.Library,
	ref domain.TrackRef,
) (*jobResult, error) {
	source, err := i.Tracks.FindSource(ctx, lib.ID, ref.Provider, ref.ID)
	if errors.Is(err, repositories.ErrSourceNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	err = i.Uploads.Save(ctx, &domain.Upload{
		UserID:        job.UserID,
		TrackID:       source.TrackID,
		TrackSourceID: source.ID,
	})
	if err != nil {
		return nil, err
	}
	return alreadyExists(), nil
}

// lockAndRecheck takes the Personal Library's lock until the transaction
// ends, and the Shared Library's only when the ref is there: linking its
// file must not race an unshare, while locking it for every upload would
// queue all users behind one another. Under the lock the ref may turn out
// known in the user's library, or still shared; then no download is needed.
func (i *ProcessIngestJob) lockAndRecheck(
	ctx context.Context,
	job *domain.IngestJob,
	libs libraries.UserLibraries,
	ref domain.TrackRef,
	files *libraries.FileChanges,
) (*jobResult, error) {
	inShared, err := i.inLibrary(ctx, libs.Shared, ref)
	if err != nil {
		return nil, err
	}
	locked := []uint{libs.Personal.ID}
	if inShared {
		locked = append(locked, libs.Shared.ID)
	}
	if err := i.Lock.Lock(ctx, locked...); err != nil {
		return nil, err
	}

	if result, err := i.knownSource(ctx, job, libs.Personal, ref); result != nil || err != nil {
		return result, err
	}
	if !inShared {
		return nil, nil
	}

	source, err := i.Tracks.FindSource(ctx, libs.Shared.ID, ref.Provider, ref.ID)
	if errors.Is(err, repositories.ErrSourceNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	track, err := i.Tracks.Get(ctx, source.TrackID)
	if err != nil {
		return nil, err
	}
	return i.linkShared(ctx, job, libs, ref, track, source.TelegramFile(), files)
}

func (i *ProcessIngestJob) inLibrary(ctx context.Context, library *domain.Library, ref domain.TrackRef) (bool, error) {
	_, err := i.Tracks.FindSource(ctx, library.ID, ref.Provider, ref.ID)
	if errors.Is(err, repositories.ErrSourceNotFound) {
		return false, nil
	}
	return err == nil, err
}

// linkShared stores the very file somebody shared by linking it instead of
// downloading it again. It is no Take: the user found the track on their
// own. A Duplicate the user already owns only gets the Upload.
func (i *ProcessIngestJob) linkShared(
	ctx context.Context,
	job *domain.IngestJob,
	libs libraries.UserLibraries,
	ref domain.TrackRef,
	shared *domain.Track,
	file *domain.TelegramFile,
	files *libraries.FileChanges,
) (*jobResult, error) {
	own, err := i.Tracks.FindDuplicate(ctx, libs.Personal.ID, shared.Metadata, shared.DurationMs)
	if err == nil {
		if err := i.recordUpload(ctx, job.UserID, own, ref, file); err != nil {
			return nil, err
		}
		return alreadyExists(), nil
	}
	if !errors.Is(err, repositories.ErrTrackNotFound) {
		return nil, err
	}

	copied, target, err := libraries.CopyTrack(ctx, i.Tracks, i.Disk, i.MusicDir, shared, libs.Shared, libs.Personal, files)
	if err != nil {
		return nil, err
	}
	if err := i.recordUpload(ctx, job.UserID, copied, ref, file); err != nil {
		return nil, err
	}
	return &jobResult{Outcome: IngestStored, Path: target, TrackID: copied.ID}, nil
}

// recordUpload reuses the Track's source for the ref if it has one.
func (i *ProcessIngestJob) recordUpload(
	ctx context.Context,
	userID uint,
	track *domain.Track,
	ref domain.TrackRef,
	file *domain.TelegramFile,
) error {
	source, err := i.Tracks.FindSource(ctx, track.LibraryID, ref.Provider, ref.ID)
	if errors.Is(err, repositories.ErrSourceNotFound) {
		source = newSource(track, ref, file)
		err = i.Tracks.SaveSource(ctx, source)
	}
	if err != nil {
		return err
	}
	return i.Uploads.Save(ctx, &domain.Upload{UserID: userID, TrackID: track.ID, TrackSourceID: source.ID})
}

func newSource(track *domain.Track, ref domain.TrackRef, file *domain.TelegramFile) *domain.TrackSource {
	source := &domain.TrackSource{TrackID: track.ID, LibraryID: track.LibraryID, Provider: ref.Provider, Ref: ref.ID}
	if file != nil {
		source.TelegramFileID = file.ID
		source.TelegramFileKind = file.Kind
	}
	return source
}

func (i *ProcessIngestJob) storeNew(ctx context.Context, in incoming) (*jobResult, error) {
	rel, err := i.Disk.FreePath(in.dir, domain.LayoutPath(in.meta, in.format, in.audio.FileName))
	if err != nil {
		return nil, err
	}

	// Database rows first, the file last: until the transaction commits
	// the rows are invisible, and a rollback takes the file back out.
	track := &domain.Track{LibraryID: in.library.ID, Metadata: in.meta}
	if err := i.saveFile(ctx, in, track, rel); err != nil {
		return nil, err
	}
	if err := i.addSource(ctx, in, track); err != nil {
		return nil, err
	}

	target := filepath.Join(in.dir, rel)
	if err := i.Disk.Place(in.staged, target); err != nil {
		return nil, err
	}
	in.files.OnRollback(func() { i.Disk.Remove(target) })

	outcome := IngestStored
	if !in.meta.Complete() {
		outcome = IngestStoredInInbox
	}
	return &jobResult{Outcome: outcome, Path: target, TrackID: track.ID}, nil
}

// mergeDuplicate always keeps the new source and Upload, and swaps the file
// only for better Quality. The Track keeps its metadata, so the path stays
// the same apart from the extension.
func (i *ProcessIngestJob) mergeDuplicate(ctx context.Context, in incoming, track *domain.Track) (*jobResult, error) {
	if err := i.addSource(ctx, in, track); err != nil {
		return nil, err
	}
	if !in.probe.Quality.Better(track.Quality) {
		return alreadyExists(), nil
	}

	oldRel := track.Path
	rel := domain.LayoutPath(track.Metadata, in.format, in.audio.FileName)
	if rel != oldRel {
		var err error
		if rel, err = i.Disk.FreePath(in.dir, rel); err != nil {
			return nil, err
		}
	}

	if err := i.saveFile(ctx, in, track, rel); err != nil {
		return nil, err
	}

	old := filepath.Join(in.dir, oldRel)
	target := filepath.Join(in.dir, rel)
	if rel != oldRel {
		// Both files exist until commit; only then the old one goes.
		if err := i.Disk.Place(in.staged, target); err != nil {
			return nil, err
		}
		in.files.OnRollback(func() { i.Disk.Remove(target) })
		in.files.AfterCommit(func() { i.Disk.Remove(old) })
		return &jobResult{Outcome: IngestReplaced, Path: target, TrackID: track.ID}, nil
	}

	// Same path: overwriting before commit would lose the old file if the
	// commit fails, so the new one waits in the scratch directory.
	pending := filepath.Join(filepath.Dir(in.staged), "pending-"+filepath.Base(in.staged))
	if err := i.Disk.Place(in.staged, pending); err != nil {
		return nil, err
	}
	in.files.OnRollback(func() { i.Disk.Remove(pending) })
	in.files.AfterCommit(func() { i.Disk.Move(pending, target) })
	return &jobResult{Outcome: IngestReplaced, Path: target, TrackID: track.ID}, nil
}

// saveFile writes the Track's final tags into the staged file, so any
// Navidrome client shows the same metadata the database holds.
func (i *ProcessIngestJob) saveFile(ctx context.Context, in incoming, track *domain.Track, rel string) error {
	track.Path = rel
	track.Format = in.format
	track.DurationMs = in.probe.DurationMs
	track.Quality = in.probe.Quality

	if err := i.Tags.WriteTags(in.staged, track.Metadata); err != nil {
		return err
	}
	if in.audio.Cover != nil {
		if err := i.Tags.WriteCover(in.staged, in.audio.Cover); err != nil {
			return err
		}
	}
	return i.Tracks.SaveTrack(ctx, track)
}

// addSource keeps the Telegram file_id, which lets inline mode resend the
// track instantly even after its file was replaced.
func (i *ProcessIngestJob) addSource(ctx context.Context, in incoming, track *domain.Track) error {
	source := newSource(track, in.ref, in.audio.TelegramFile)
	if err := i.Tracks.SaveSource(ctx, source); err != nil {
		return err
	}

	return i.Uploads.Save(ctx, &domain.Upload{
		UserID:        in.job.UserID,
		TrackID:       track.ID,
		TrackSourceID: source.ID,
	})
}

// stage copies the Provider's stream into a scratch file, so the following
// steps can read and tag a local file.
func (i *ProcessIngestJob) stage(audio *providers.FetchedAudio) (string, error) {
	defer audio.Body.Close()
	return i.Disk.Stage(audio.Body, audio.Format.Ext())
}

type step string

const (
	stepSource step = "source"
	stepFetch  step = "fetch"
	stepStage  step = "stage"
	stepRemux  step = "remux"
	stepProbe  step = "probe"
	stepStore  step = "store"
)

type stepError struct {
	step step
	err  error
}

func (e *stepError) Error() string { return string(e.step) + ": " + e.err.Error() }

func (e *stepError) Unwrap() error { return e.err }

func wrapStep(s step, err error) error {
	if err == nil {
		return nil
	}
	return &stepError{step: s, err: err}
}
