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
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/ingest"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
)

type ProcessIngestJob struct {
	Tx          repositories.TxManager
	Queue       repositories.IngestQueue
	Providers   *providers.Registry
	Tracks      repositories.Tracks
	Uploads     UploadRepository
	Libraries   repositories.Libraries
	Lock        repositories.LibraryLock
	Disk        common.Disk
	Tags        AudioTags
	Remuxer     Remuxer
	Batches     repositories.IngestBatches
	InFlight    *InFlight
	MusicDir    string
	RetryDelays []time.Duration
}

type UploadRepository interface {
	Save(ctx context.Context, upload *library.Upload) error
}

type AudioTags interface {
	// Probe returns ErrCorruptAudio for unreadable tags or no duration.
	Probe(path string) (*AudioProbe, error)
	WriteTags(path string, m library.Metadata) error
	WriteCover(path string, image []byte) error
}

type AudioProbe struct {
	Tags       library.Metadata
	DurationMs int
	Quality    library.Quality
}

type Remuxer interface {
	// Codec returns "" if it cannot tell.
	Codec(ctx context.Context, path string) string
	WavToFlac(ctx context.Context, wav string) (string, error)
	// UnpackFlac takes FLAC out of MP4 without re-encoding.
	UnpackFlac(ctx context.Context, mp4 string) (string, error)
}

var ErrCorruptAudio = errors.New("audio file is corrupt")

// InFlight counts jobs from claim to the end of their bookkeeping; empty polls don't
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
	job    *ingest.IngestJob
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

		i.recordAttempt(a.job, a.result, a.err)
		return i.Queue.Save(ctx, a.job)
	})
	return a, err
}

func (i *ProcessIngestJob) finish(ctx context.Context, a attempt) {
	if a.job.Status != ingest.IngestJobPending {
		if err := i.release(ctx, a.job); err != nil {
			slog.Error("ingest_job_release_failed", "job_id", a.job.ID, "error", err)
		}
	}

	if a.job.Status == ingest.IngestJobDone {
		slog.Info("ingest_job_done", "job_id", a.job.ID, "outcome", a.result.Outcome, "path", a.result.Path)
	}

	if a.job.Status == ingest.IngestJobPending || a.job.BatchID == nil {
		return
	}
	if err := finishBatch(ctx, i.Queue, i.Batches, *a.job.BatchID); err != nil {
		slog.Error("finish_ingest_batch", "batch_id", *a.job.BatchID, "error", err)
	}
}

func (i *ProcessIngestJob) recordAttempt(job *ingest.IngestJob, result *jobResult, procErr error) {
	if procErr == nil {
		job.Status = ingest.IngestJobDone
		job.Outcome = result.Outcome
		job.TrackID = &result.TrackID
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
		job.Status = ingest.IngestJobFailed
		job.FailureReason = failureReason(procErr)
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

func failureReason(err error) ingest.FailureReason {
	var permanent *providers.PermanentError
	if errors.As(err, &permanent) {
		return permanent.Reason
	}
	if failedStep(err) == stepFetch {
		return ingest.ReasonFetchFailed
	}
	return ingest.ReasonInternal
}

type jobResult struct {
	Outcome library.Outcome
	Path    string
	TrackID uint
}

func alreadyExists(trackID uint) *jobResult {
	return &jobResult{Outcome: library.AlreadyExists, TrackID: trackID}
}

// incoming carries what process learned about the audio down to the store step.
type incoming struct {
	job    *ingest.IngestJob
	lib    *library.Library
	dir    string
	ref    provider.TrackRef
	audio  *providers.FetchedAudio
	format library.Format
	probe  *AudioProbe
	meta   library.Metadata
	staged string
	files  *libraries.FileChanges
}

// process turns the job's audio into a Track. Database writes go through
// ctx's transaction; the file is moved into the Library last, and files
// records how to make the Library follow the transaction's outcome.
func (i *ProcessIngestJob) process(ctx context.Context, job *ingest.IngestJob, files *libraries.FileChanges) (*jobResult, error) {
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
	// neither does the very file somebody shared or the service gave out.
	if result, err := i.knownSource(ctx, job, personal, ref); result != nil || err != nil {
		return result, wrapStep(stepSource, err)
	}
	inShared, err := i.inLibrary(ctx, shared, ref)
	if err != nil {
		return nil, wrapStep(stepSource, err)
	}
	recognized, err := i.recognize(ctx, libs, ref)
	if err != nil {
		return nil, wrapStep(stepSource, err)
	}
	if inShared || recognized.any() {
		if result, err := i.lockAndRecheck(ctx, job, libs, ref, files); result != nil || err != nil {
			return result, wrapStep(stepSource, err)
		}
	}

	// A Provider without the Capability will not grow it on retry.
	fetcher, err := i.Providers.Fetcher(ref.Provider)
	if err != nil {
		return nil, wrapStep(stepFetch, providers.Permanent(ingest.ReasonInternal, err))
	}

	audio, err := fetcher.Fetch(ctx, job.UserID, ref)
	if errors.Is(err, repositories.ErrProviderAccountNotFound) {
		return nil, wrapStep(stepFetch, providers.Permanent(ingest.ReasonNoProviderAccount, err))
	}
	if errors.Is(err, providers.ErrUnauthorized) {
		return nil, wrapStep(stepFetch, providers.Permanent(ingest.ReasonTokenRejected, err))
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
	case format == library.FormatWAV:
		remux = i.Remuxer.WavToFlac
	case format == library.FormatM4A && i.Remuxer.Codec(ctx, staged) == "flac":
		remux = i.Remuxer.UnpackFlac
	}
	if remux != nil {
		if staged, err = remux(ctx, staged); err != nil {
			return nil, wrapStep(stepRemux, err)
		}
		defer i.Disk.Remove(staged)
		format = library.FormatFLAC
	}

	probe, err := i.Tags.Probe(staged)
	if errors.Is(err, ErrCorruptAudio) {
		return nil, wrapStep(stepProbe, providers.Permanent(ingest.ReasonCorruptFile, err))
	}
	if err != nil {
		return nil, wrapStep(stepProbe, err)
	}

	// Each field comes from the first source that has it.
	meta := audio.Hint.Merge(
		probe.Tags,
		audio.WeakHint,
		library.MetadataFromFileName(audio.FileName),
	).Normalize()

	// Workers download and probe in parallel, but the duplicate check and
	// path choice run one at a time per Library: otherwise two workers could
	// both miss the duplicate and store the same track twice. Another job may
	// also have added this source while we were fetching.
	if result, err := i.lockAndRecheck(ctx, job, libs, ref, files); result != nil || err != nil {
		return result, wrapStep(stepSource, err)
	}

	in := incoming{
		job:    job,
		lib:    personal,
		dir:    libraries.Dir(i.MusicDir, personal),
		ref:    ref,
		audio:  audio,
		format: format,
		probe:  probe,
		meta:   meta,
		staged: staged,
		files:  files,
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
func (i *ProcessIngestJob) release(ctx context.Context, job *ingest.IngestJob) error {
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
	job *ingest.IngestJob,
	lib *library.Library,
	ref provider.TrackRef,
) (*jobResult, error) {
	source, err := i.Tracks.FindSource(ctx, lib.ID, ref.Provider, ref.ID)
	if errors.Is(err, repositories.ErrSourceNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	err = i.Uploads.Save(ctx, &library.Upload{
		UserID:        job.UserID,
		TrackID:       source.TrackID,
		TrackSourceID: source.ID,
	})
	if err != nil {
		return nil, err
	}
	return alreadyExists(source.TrackID), nil
}

type recognition struct {
	own    *library.Track
	shared *library.Track
}

func (r recognition) any() bool {
	return r.own != nil || r.shared != nil
}

// recognize asks the Provider whether the ref is a file the service gave
// out; the Tracks of other users' libraries stay unseen.
func (i *ProcessIngestJob) recognize(ctx context.Context, libs libraries.UserLibraries, ref provider.TrackRef) (recognition, error) {
	recognizer, err := i.Providers.Recognizer(ref.Provider)
	if errors.Is(err, providers.ErrCapabilityNotSupported) {
		return recognition{}, nil
	}
	if err != nil {
		return recognition{}, err
	}
	ids, err := recognizer.Recognize(ctx, ref)
	if err != nil {
		return recognition{}, err
	}

	var r recognition
	for _, id := range ids {
		track, err := i.Tracks.Get(ctx, id)
		if errors.Is(err, repositories.ErrTrackNotFound) {
			continue
		}
		if err != nil {
			return recognition{}, err
		}
		switch {
		case r.own == nil && track.In(libs.Personal):
			r.own = track
		case r.shared == nil && track.In(libs.Shared):
			r.shared = track
		}
	}
	return r, nil
}

// lockAndRecheck takes the Personal Library's lock until the transaction
// ends, and the Shared Library's only when the ref is there: linking its
// file must not race an unshare, while locking it for every upload would
// queue all users behind one another. Under the lock the ref may turn out
// known or recognized in the user's library, or still shared; then no
// download is needed.
func (i *ProcessIngestJob) lockAndRecheck(
	ctx context.Context,
	job *ingest.IngestJob,
	libs libraries.UserLibraries,
	ref provider.TrackRef,
	files *libraries.FileChanges,
) (*jobResult, error) {
	inShared, err := i.inLibrary(ctx, libs.Shared, ref)
	if err != nil {
		return nil, err
	}
	recognized, err := i.recognize(ctx, libs, ref)
	if err != nil {
		return nil, err
	}
	inShared = inShared || recognized.shared != nil
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
	if recognized.own != nil {
		if err := i.recordUpload(ctx, job.UserID, recognized.own, ref); err != nil {
			return nil, err
		}
		return alreadyExists(recognized.own.ID), nil
	}
	if !inShared {
		return nil, nil
	}

	shared, err := i.stillShared(ctx, libs.Shared, ref, recognized.shared)
	if err != nil || shared == nil {
		return nil, err
	}
	return i.linkShared(ctx, job, libs, ref, shared, files)
}

// stillShared looks the Track up again under the lock: it may have been
// unshared meanwhile.
func (i *ProcessIngestJob) stillShared(
	ctx context.Context,
	shared *library.Library,
	ref provider.TrackRef,
	recognized *library.Track,
) (*library.Track, error) {
	trackID := uint(0)
	source, err := i.Tracks.FindSource(ctx, shared.ID, ref.Provider, ref.ID)
	switch {
	case err == nil:
		trackID = source.TrackID
	case !errors.Is(err, repositories.ErrSourceNotFound):
		return nil, err
	case recognized != nil:
		trackID = recognized.ID
	default:
		return nil, nil
	}
	track, err := i.Tracks.Get(ctx, trackID)
	if errors.Is(err, repositories.ErrTrackNotFound) {
		return nil, nil
	}
	return track, err
}

func (i *ProcessIngestJob) inLibrary(ctx context.Context, lib *library.Library, ref provider.TrackRef) (bool, error) {
	_, err := i.Tracks.FindSource(ctx, lib.ID, ref.Provider, ref.ID)
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
	job *ingest.IngestJob,
	libs libraries.UserLibraries,
	ref provider.TrackRef,
	shared *library.Track,
	files *libraries.FileChanges,
) (*jobResult, error) {
	own, err := i.Tracks.FindDuplicate(ctx, libs.Personal.ID, shared.Metadata, shared.DurationMs)
	if err == nil {
		if err := i.recordUpload(ctx, job.UserID, own, ref); err != nil {
			return nil, err
		}
		return alreadyExists(own.ID), nil
	}
	if !errors.Is(err, repositories.ErrTrackNotFound) {
		return nil, err
	}

	copied, target, err := libraries.CopyTrack(ctx, i.Tracks, i.Disk, i.MusicDir, shared, libs.Shared, libs.Personal, files)
	if err != nil {
		return nil, err
	}
	if err := i.recordUpload(ctx, job.UserID, copied, ref); err != nil {
		return nil, err
	}
	return &jobResult{Outcome: library.Stored, Path: target, TrackID: copied.ID}, nil
}

// recordUpload gives the Track a source for the ref unless it has one: the
// ref is how this user sent it.
func (i *ProcessIngestJob) recordUpload(ctx context.Context, userID uint, track *library.Track, ref provider.TrackRef) error {
	source, err := i.Tracks.FindSource(ctx, track.LibraryID, ref.Provider, ref.ID)
	if errors.Is(err, repositories.ErrSourceNotFound) {
		source = newSource(track, ref)
		err = i.Tracks.SaveSource(ctx, source)
	}
	if err != nil {
		return err
	}
	return i.Uploads.Save(ctx, &library.Upload{UserID: userID, TrackID: track.ID, TrackSourceID: source.ID})
}

func newSource(track *library.Track, ref provider.TrackRef) *library.TrackSource {
	return &library.TrackSource{TrackID: track.ID, LibraryID: track.LibraryID, Provider: ref.Provider, Ref: ref.ID}
}

func (i *ProcessIngestJob) storeNew(ctx context.Context, in incoming) (*jobResult, error) {
	rel, err := i.Disk.FreePath(in.dir, library.LayoutPath(in.meta, in.format, in.audio.FileName))
	if err != nil {
		return nil, err
	}

	// Database rows first, the file last: until the transaction commits
	// the rows are invisible, and a rollback takes the file back out.
	track := &library.Track{LibraryID: in.lib.ID, Metadata: in.meta}
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

	outcome := library.Stored
	if !in.meta.Complete() {
		outcome = library.StoredInInbox
	}
	return &jobResult{Outcome: outcome, Path: target, TrackID: track.ID}, nil
}

// mergeDuplicate always keeps the new source and Upload, and swaps the file
// only for better Quality. The Track keeps its metadata, so the path stays
// the same apart from the extension.
func (i *ProcessIngestJob) mergeDuplicate(ctx context.Context, in incoming, track *library.Track) (*jobResult, error) {
	if err := i.addSource(ctx, in, track); err != nil {
		return nil, err
	}
	if !in.probe.Quality.Better(track.Quality) {
		return alreadyExists(track.ID), nil
	}

	oldRel := track.Path
	rel := library.LayoutPath(track.Metadata, in.format, in.audio.FileName)
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
		return &jobResult{Outcome: library.Replaced, Path: target, TrackID: track.ID}, nil
	}

	// Same path: overwriting before commit would lose the old file if the
	// commit fails, so the new one waits in the scratch directory.
	pending := filepath.Join(filepath.Dir(in.staged), "pending-"+filepath.Base(in.staged))
	if err := i.Disk.Place(in.staged, pending); err != nil {
		return nil, err
	}
	in.files.OnRollback(func() { i.Disk.Remove(pending) })
	in.files.AfterCommit(func() { i.Disk.Move(pending, target) })
	return &jobResult{Outcome: library.Replaced, Path: target, TrackID: track.ID}, nil
}

// saveFile writes the Track's final tags into the staged file, so any
// Navidrome client shows the same metadata the database holds.
func (i *ProcessIngestJob) saveFile(ctx context.Context, in incoming, track *library.Track, rel string) error {
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

func (i *ProcessIngestJob) addSource(ctx context.Context, in incoming, track *library.Track) error {
	source := newSource(track, in.ref)
	if err := i.Tracks.SaveSource(ctx, source); err != nil {
		return err
	}

	return i.Uploads.Save(ctx, &library.Upload{
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
