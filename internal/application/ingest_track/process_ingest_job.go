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
	Tx        repositories.TxManager
	Queue     repositories.IngestQueue
	Providers *providers.Registry
	Tracks    repositories.Tracks
	Uploads   UploadRepository
	Libraries repositories.Libraries
	Attached  *libraries.Attached
	Lock      repositories.LibraryLock
	Disk      common.Disk
	Tags      AudioTags
	Remuxer   Remuxer
	Batches   repositories.IngestBatches
	InFlight  *InFlight
	MusicDir  string
	Retry     ingest.RetryPolicy
}

type UploadRepository interface {
	Save(ctx context.Context, upload *library.Upload) error
}

type AudioTags interface {
	// Probe returns ErrCorruptAudio for unreadable tags or no duration.
	Probe(path string) (*library.Probe, error)
	WriteTags(path string, m library.Metadata) error
	WriteCover(path string, image []byte) error
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

	files := libraries.NewFileChanges(i.Disk)
	a, err := i.try(ctx, filter, files)
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
	if a.job.Finished() {
		if err := i.release(ctx, a.job); err != nil {
			slog.Error("ingest_job_release_failed", "job_id", a.job.ID, "error", err)
		}
	}

	if a.job.Done() {
		slog.Info("ingest_job_done", "job_id", a.job.ID, "outcome", a.result.Outcome, "path", a.result.Path)
	}

	if !a.job.Finished() || a.job.BatchID == nil {
		return
	}
	if err := finishBatch(ctx, i.Tx, i.Queue, i.Batches, *a.job.BatchID); err != nil {
		slog.Error("finish_ingest_batch", "batch_id", *a.job.BatchID, "error", err)
	}
}

func (i *ProcessIngestJob) recordAttempt(job *ingest.IngestJob, result *jobResult, procErr error) {
	if procErr == nil {
		job.Succeed(result.Outcome, result.TrackID)
		return
	}

	reason, permanent := failureReason(procErr)
	final := job.Fail(reason, procErr, permanent, i.Retry, time.Now())

	slog.Error(
		"ingest_job_failed",
		"job_id", job.ID,
		"step", failedStep(procErr),
		"attempt", job.Attempts,
		"final", final,
		"error", procErr,
	)
}

func failedStep(err error) step {
	var se *stepError
	if errors.As(err, &se) {
		return se.step
	}
	return "unknown"
}

// failureReason also tells whether retrying cannot help.
func failureReason(err error) (reason ingest.FailureReason, permanent bool) {
	var p *providers.PermanentError
	if errors.As(err, &p) {
		return p.Reason, true
	}
	if failedStep(err) == stepFetch {
		return ingest.ReasonFetchFailed, false
	}
	return ingest.ReasonInternal, false
}

type jobResult struct {
	Outcome library.Outcome
	Path    string
	TrackID uint
}

func alreadyExists(trackID uint) *jobResult {
	return &jobResult{Outcome: library.AlreadyExists, TrackID: trackID}
}

type storing struct {
	job    *ingest.IngestJob
	lib    *library.Library
	dir    string
	cover  []byte
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
	libs := libraries.ManagedLibraries{Personal: personal, Shared: shared}

	attached, err := i.Attached.VisibleTo(ctx, job.UserID)
	if err != nil {
		return nil, wrapStep(stepSource, err)
	}
	// A forwarded copy of a known file needs no download at all, and
	// neither does the very file somebody shared or the service gave out.
	if result, err := i.knownSource(ctx, job, library.KeptLibraries(personal, attached), ref); result != nil || err != nil {
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

	if result, err := i.attachedByDescription(ctx, job, attached, ref); result != nil || err != nil {
		return result, wrapStep(stepSource, err)
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

	in := library.NewIncoming(ref, audio.Hint, audio.WeakHint, audio.FileName, format, *probe)

	// Workers download and probe in parallel, but the duplicate check and
	// path choice run one at a time per Library: otherwise two workers could
	// both miss the duplicate and store the same track twice. Another job may
	// also have added this source while we were fetching.
	if result, err := i.lockAndRecheck(ctx, job, libs, ref, files); result != nil || err != nil {
		return result, wrapStep(stepSource, err)
	}

	st := storing{
		job:    job,
		lib:    personal,
		dir:    libraries.Dir(i.MusicDir, personal),
		cover:  audio.Cover,
		staged: staged,
		files:  files,
	}
	attachedTrack, err := i.attachedDuplicate(ctx, attached, in.Metadata, in.DurationMs)
	if err != nil {
		return nil, wrapStep(stepStore, err)
	}
	if attachedTrack != nil {
		result, err := i.mergeDuplicate(ctx, st, attachedTrack, in)
		return result, wrapStep(stepStore, err)
	}
	duplicate, err := i.Tracks.FindDuplicate(ctx, []uint{personal.ID}, in.Metadata, in.DurationMs)
	switch {
	case err == nil:
		result, err := i.mergeDuplicate(ctx, st, duplicate, in)
		return result, wrapStep(stepStore, err)
	case !errors.Is(err, repositories.ErrTrackNotFound):
		return nil, wrapStep(stepStore, err)
	}
	result, err := i.storeNew(ctx, st, in)
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
	libs []*library.Library,
	ref provider.TrackRef,
) (*jobResult, error) {
	source, err := i.Tracks.FindSource(ctx, libraries.IDs(libs), ref.Provider, ref.ID)
	if errors.Is(err, repositories.ErrSourceNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if err := i.Uploads.Save(ctx, library.NewUpload(job.UserID, source)); err != nil {
		return nil, err
	}
	return alreadyExists(source.TrackID), nil
}

// attachedByDescription spares the download of a track an Attached Library the
// user sees has already, by what the Provider tells of it. Without a
// description the audio is fetched and checked by its tags.
func (i *ProcessIngestJob) attachedByDescription(
	ctx context.Context,
	job *ingest.IngestJob,
	attached []*library.Library,
	ref provider.TrackRef,
) (*jobResult, error) {
	if len(attached) == 0 {
		return nil, nil
	}
	describer, err := i.Providers.Describer(ref.Provider)
	if errors.Is(err, providers.ErrCapabilityNotSupported) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	described, err := describer.Describe(ctx, job.UserID, ref)
	if err != nil {
		slog.Warn("describe_track", "job_id", job.ID, "error", err)
		return nil, nil
	}
	if described == nil {
		return nil, nil
	}
	attachedTrack, err := i.attachedDuplicate(ctx, attached, described.Metadata.Normalize(), described.DurationMs)
	if err != nil || attachedTrack == nil {
		return nil, err
	}
	if err := i.recordUpload(ctx, job.UserID, attachedTrack, ref); err != nil {
		return nil, err
	}
	return alreadyExists(attachedTrack.ID), nil
}

// attachedDuplicate finds the audio in the Attached Libraries and locks the
// one holding it, as the Track gains a Source there. It returns nil if none
// has it.
func (i *ProcessIngestJob) attachedDuplicate(
	ctx context.Context,
	attached []*library.Library,
	m library.Metadata,
	durationMs int,
) (*library.Track, error) {
	found, err := i.Tracks.FindDuplicate(ctx, libraries.IDs(attached), m, durationMs)
	if errors.Is(err, repositories.ErrTrackNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	// Found again under the lock: the song may have gone meanwhile.
	if err := i.Lock.Lock(ctx, found.LibraryID); err != nil {
		return nil, err
	}
	track, err := i.Tracks.FindDuplicate(ctx, []uint{found.LibraryID}, m, durationMs)
	if errors.Is(err, repositories.ErrTrackNotFound) {
		return nil, nil
	}
	return track, err
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
func (i *ProcessIngestJob) recognize(ctx context.Context, libs libraries.ManagedLibraries, ref provider.TrackRef) (recognition, error) {
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

	tracks, err := i.Tracks.GetMany(ctx, ids)
	if err != nil {
		return recognition{}, err
	}
	var r recognition
	for n := range tracks {
		track := &tracks[n]
		switch {
		case r.own == nil && track.In(libs.Personal):
			r.own = track
		case r.shared == nil && track.In(libs.Shared):
			r.shared = track
		}
	}
	return r, nil
}

// Lock the Shared Library only when the ref is shared, preventing an unshare
// from racing the link without serializing unrelated uploads. Recheck under
// lock because another job may have added or unshared the track meanwhile.
func (i *ProcessIngestJob) lockAndRecheck(
	ctx context.Context,
	job *ingest.IngestJob,
	libs libraries.ManagedLibraries,
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

	if result, err := i.knownSource(ctx, job, []*library.Library{libs.Personal}, ref); result != nil || err != nil {
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
	var trackID uint
	source, err := i.Tracks.FindSource(ctx, []uint{shared.ID}, ref.Provider, ref.ID)
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
	_, err := i.Tracks.FindSource(ctx, []uint{lib.ID}, ref.Provider, ref.ID)
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
	libs libraries.ManagedLibraries,
	ref provider.TrackRef,
	shared *library.Track,
	files *libraries.FileChanges,
) (*jobResult, error) {
	own, err := i.Tracks.FindDuplicate(ctx, []uint{libs.Personal.ID}, shared.Metadata, shared.DurationMs)
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

// recordUpload: the ref is how this user sent the Track.
func (i *ProcessIngestJob) recordUpload(ctx context.Context, userID uint, track *library.Track, ref provider.TrackRef) error {
	source := track.AddSource(ref)
	if source.ID == 0 {
		if err := i.Tracks.SaveTrack(ctx, track); err != nil {
			return err
		}
	}
	return i.Uploads.Save(ctx, library.NewUpload(userID, source))
}

func (i *ProcessIngestJob) storeNew(ctx context.Context, st storing, in library.Incoming) (*jobResult, error) {
	track, outcome := library.NewTrack(st.lib, in)
	if err := i.freePath(st, track, ""); err != nil {
		return nil, err
	}

	// Database rows first, the file last: until the transaction commits
	// the rows are invisible, and a rollback takes the file back out.
	if err := i.saveFile(ctx, st, track); err != nil {
		return nil, err
	}
	if err := i.recordUpload(ctx, st.job.UserID, track, in.Ref); err != nil {
		return nil, err
	}

	target := filepath.Join(st.dir, track.Path)
	if err := st.files.Place(st.staged, target); err != nil {
		return nil, err
	}
	return &jobResult{Outcome: outcome, Path: target, TrackID: track.ID}, nil
}

func (i *ProcessIngestJob) mergeDuplicate(ctx context.Context, st storing, track *library.Track, in library.Incoming) (*jobResult, error) {
	old := track.Path
	if track.Absorb(in) == library.AlreadyExists {
		if err := i.recordUpload(ctx, st.job.UserID, track, in.Ref); err != nil {
			return nil, err
		}
		return alreadyExists(track.ID), nil
	}

	if err := i.freePath(st, track, old); err != nil {
		return nil, err
	}
	if err := i.saveFile(ctx, st, track); err != nil {
		return nil, err
	}
	if err := i.recordUpload(ctx, st.job.UserID, track, in.Ref); err != nil {
		return nil, err
	}

	target := filepath.Join(st.dir, track.Path)
	if err := st.files.Replace(st.staged, filepath.Join(st.dir, old), target); err != nil {
		return nil, err
	}
	return &jobResult{Outcome: library.Replaced, Path: target, TrackID: track.ID}, nil
}

// freePath moves the Track off a path another file takes. own is the path
// of the Track's current file, which it may keep.
func (i *ProcessIngestJob) freePath(st storing, track *library.Track, own string) error {
	if track.Path == own {
		return nil
	}
	rel, err := i.Disk.FreePath(st.dir, track.Path)
	if err != nil {
		return err
	}
	track.MoveTo(rel)
	return nil
}

// saveFile writes the Track's final tags into the staged file, so any
// Navidrome client shows the same metadata the database holds.
func (i *ProcessIngestJob) saveFile(ctx context.Context, st storing, track *library.Track) error {
	if err := i.Tags.WriteTags(st.staged, track.Metadata); err != nil {
		return err
	}
	if st.cover != nil {
		if err := i.Tags.WriteCover(st.staged, st.cover); err != nil {
			return err
		}
	}
	return i.Tracks.SaveTrack(ctx, track)
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
