package ingest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/lubaskinc0de/navidrome-tg/internal/application"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

// scratchDir lives inside the Library so the final rename is atomic;
// Navidrome skips hidden directories.
const scratchDir = ".navidrome-tg"

type Pipeline struct {
	Providers *application.Providers
	Tracks    application.TrackRepository
	Uploads   application.UploadRepository
	Libraries application.LibraryRepository
	Shared    SharedCopier
	Lock      application.LibraryLock
	MusicDir  string
}

type SharedCopier interface {
	CopyShared(
		ctx context.Context,
		track *domain.Track,
		libs application.UserLibraries,
		files *library.FileChanges,
	) (copied *domain.Track, target string, err error)
}

type Result struct {
	Outcome application.IngestOutcome
	Path    string
}

func alreadyExists() *Result {
	return &Result{Outcome: application.IngestAlreadyExists}
}

// incoming carries what Process learned about the audio down to the store step.
type incoming struct {
	job     *domain.IngestJob
	library *domain.Library
	dir     string
	ref     domain.TrackRef
	audio   *application.FetchedAudio
	format  domain.Format
	probe   *probe
	meta    domain.Metadata
	staged  string
	files   *library.FileChanges
}

// Process turns the job's audio into a Track. Database writes go through
// ctx's transaction; the file is moved into the Library last, and files
// records how to make the Library follow the transaction's outcome.
func (p *Pipeline) Process(ctx context.Context, job *domain.IngestJob, files *library.FileChanges) (*Result, error) {
	ref := job.Ref()

	personal, err := p.Libraries.Personal(ctx, job.UserID)
	if err != nil {
		return nil, wrapStep(stepSource, err)
	}
	shared, err := p.Libraries.Shared(ctx)
	if err != nil {
		return nil, wrapStep(stepSource, err)
	}
	libs := application.UserLibraries{Personal: personal, Shared: shared}

	// A forwarded copy of a known file needs no download at all, and
	// neither does the very file somebody shared.
	if result, err := p.knownSource(ctx, job, personal, ref); result != nil || err != nil {
		return result, wrapStep(stepSource, err)
	}
	inShared, err := p.inLibrary(ctx, shared, ref)
	if err != nil {
		return nil, wrapStep(stepSource, err)
	}
	if inShared {
		if result, err := p.lockAndRecheck(ctx, job, libs, ref, files); result != nil || err != nil {
			return result, wrapStep(stepSource, err)
		}
	}

	// A Provider without the Capability will not grow it on retry.
	fetcher, err := p.Providers.Fetcher(ref.Provider)
	if err != nil {
		return nil, wrapStep(stepFetch, application.Permanent(application.ReasonInternal, err))
	}

	audio, err := fetcher.Fetch(ctx, ref)
	if err != nil {
		return nil, wrapStep(stepFetch, err)
	}

	// From here on the pipeline works on its own copy; the deferred removal
	// is a no-op once the file has been moved into the Library.
	staged, err := p.stage(audio)
	if err != nil {
		return nil, wrapStep(stepStage, err)
	}
	defer os.Remove(staged)

	// WAV carries no proper tags, so it is repacked into lossless FLAC first.
	format := audio.Format
	if format == domain.FormatWAV {
		if staged, err = wavToFlac(ctx, staged); err != nil {
			return nil, wrapStep(stepRemux, err)
		}
		defer os.Remove(staged)
		format = domain.FormatFLAC
	}

	probe, err := probeFile(staged)
	if err != nil {
		return nil, wrapStep(stepProbe, err)
	}

	// Each field comes from the first source that has it.
	meta := audio.Hint.Merge(
		probe.tags,
		audio.WeakHint,
		domain.MetadataFromFileName(audio.FileName),
	).Normalize()

	// Workers download and probe in parallel, but the duplicate check and
	// path choice run one at a time per Library: otherwise two workers could
	// both miss the duplicate and store the same track twice. Another job may
	// also have added this source while we were fetching.
	if result, err := p.lockAndRecheck(ctx, job, libs, ref, files); result != nil || err != nil {
		return result, wrapStep(stepSource, err)
	}

	in := incoming{
		job:     job,
		library: personal,
		dir:     filepath.Join(p.MusicDir, personal.Dir),
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
		duplicate, err := p.Tracks.FindDuplicate(ctx, personal.ID, meta, probe.durationMs)
		switch {
		case err == nil:
			result, err := p.mergeDuplicate(ctx, in, duplicate)
			return result, wrapStep(stepStore, err)
		case !errors.Is(err, application.ErrTrackNotFound):
			return nil, wrapStep(stepStore, err)
		}
	}

	result, err := p.storeNew(ctx, in)
	return result, wrapStep(stepStore, err)
}

// Release is called once the job is finished for good; Providers that keep
// nothing between attempts simply lack the Capability.
func (p *Pipeline) Release(ctx context.Context, job *domain.IngestJob) error {
	releaser, err := p.Providers.Releaser(job.Provider)
	if errors.Is(err, application.ErrCapabilityNotSupported) {
		return nil
	}
	if err != nil {
		return err
	}
	return releaser.Release(ctx, job.Ref())
}

// knownSource records the Upload for a Track Ref already stored in the
// library, or returns nil when the ref is new there.
func (p *Pipeline) knownSource(
	ctx context.Context,
	job *domain.IngestJob,
	lib *domain.Library,
	ref domain.TrackRef,
) (*Result, error) {
	source, err := p.Tracks.FindSource(ctx, lib.ID, ref.Provider, ref.ID)
	if errors.Is(err, application.ErrSourceNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	err = p.Uploads.Save(ctx, &domain.Upload{
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
func (p *Pipeline) lockAndRecheck(
	ctx context.Context,
	job *domain.IngestJob,
	libs application.UserLibraries,
	ref domain.TrackRef,
	files *library.FileChanges,
) (*Result, error) {
	inShared, err := p.inLibrary(ctx, libs.Shared, ref)
	if err != nil {
		return nil, err
	}
	locked := []uint{libs.Personal.ID}
	if inShared {
		locked = append(locked, libs.Shared.ID)
	}
	if err := p.Lock.Lock(ctx, locked...); err != nil {
		return nil, err
	}

	if result, err := p.knownSource(ctx, job, libs.Personal, ref); result != nil || err != nil {
		return result, err
	}
	if !inShared {
		return nil, nil
	}

	source, err := p.Tracks.FindSource(ctx, libs.Shared.ID, ref.Provider, ref.ID)
	if errors.Is(err, application.ErrSourceNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	track, err := p.Tracks.Get(ctx, source.TrackID)
	if err != nil {
		return nil, err
	}
	return p.linkShared(ctx, job, libs, ref, track, source.TelegramFile(), files)
}

func (p *Pipeline) inLibrary(ctx context.Context, library *domain.Library, ref domain.TrackRef) (bool, error) {
	_, err := p.Tracks.FindSource(ctx, library.ID, ref.Provider, ref.ID)
	if errors.Is(err, application.ErrSourceNotFound) {
		return false, nil
	}
	return err == nil, err
}

// linkShared stores the very file somebody shared by linking it instead of
// downloading it again. It is no Take: the user found the track on their
// own. A Duplicate the user already owns only gets the Upload.
func (p *Pipeline) linkShared(
	ctx context.Context,
	job *domain.IngestJob,
	libs application.UserLibraries,
	ref domain.TrackRef,
	shared *domain.Track,
	file *domain.TelegramFile,
	files *library.FileChanges,
) (*Result, error) {
	own, err := p.Tracks.FindDuplicate(ctx, libs.Personal.ID, shared.Metadata, shared.DurationMs)
	if err == nil {
		if err := p.recordUpload(ctx, job.UserID, own, ref, file); err != nil {
			return nil, err
		}
		return alreadyExists(), nil
	}
	if !errors.Is(err, application.ErrTrackNotFound) {
		return nil, err
	}

	copied, target, err := p.Shared.CopyShared(ctx, shared, libs, files)
	if err != nil {
		return nil, err
	}
	if err := p.recordUpload(ctx, job.UserID, copied, ref, file); err != nil {
		return nil, err
	}
	return &Result{Outcome: application.IngestStored, Path: target}, nil
}

// recordUpload reuses the Track's source for the ref if it has one.
func (p *Pipeline) recordUpload(
	ctx context.Context,
	userID uint,
	track *domain.Track,
	ref domain.TrackRef,
	file *domain.TelegramFile,
) error {
	source, err := p.Tracks.FindSource(ctx, track.LibraryID, ref.Provider, ref.ID)
	if errors.Is(err, application.ErrSourceNotFound) {
		source = newSource(track, ref, file)
		err = p.Tracks.SaveSource(ctx, source)
	}
	if err != nil {
		return err
	}
	return p.Uploads.Save(ctx, &domain.Upload{UserID: userID, TrackID: track.ID, TrackSourceID: source.ID})
}

func newSource(track *domain.Track, ref domain.TrackRef, file *domain.TelegramFile) *domain.TrackSource {
	source := &domain.TrackSource{TrackID: track.ID, LibraryID: track.LibraryID, Provider: ref.Provider, Ref: ref.ID}
	if file != nil {
		source.TelegramFileID = file.ID
		source.TelegramFileKind = file.Kind
	}
	return source
}

func (p *Pipeline) storeNew(ctx context.Context, in incoming) (*Result, error) {
	rel, err := library.FreePath(in.dir, layoutPath(in.meta, in.format, in.audio.FileName))
	if err != nil {
		return nil, err
	}

	// Database rows first, the file last: until the transaction commits
	// the rows are invisible, and a rollback takes the file back out.
	track := &domain.Track{LibraryID: in.library.ID, Metadata: in.meta}
	if err := p.saveFile(ctx, in, track, rel); err != nil {
		return nil, err
	}
	if err := p.addSource(ctx, in, track); err != nil {
		return nil, err
	}

	target := filepath.Join(in.dir, rel)
	if err := library.Place(in.staged, target); err != nil {
		return nil, err
	}
	in.files.OnRollback(func() { library.RemoveFile(target) })

	outcome := application.IngestStored
	if !in.meta.Complete() {
		outcome = application.IngestStoredInInbox
	}
	return &Result{Outcome: outcome, Path: target}, nil
}

// mergeDuplicate always keeps the new source and Upload, and swaps the file
// only for better Quality. The Track keeps its metadata, so the path stays
// the same apart from the extension.
func (p *Pipeline) mergeDuplicate(ctx context.Context, in incoming, track *domain.Track) (*Result, error) {
	if err := p.addSource(ctx, in, track); err != nil {
		return nil, err
	}
	if !in.probe.quality.Better(track.Quality) {
		return alreadyExists(), nil
	}

	oldRel := track.Path
	rel := layoutPath(track.Metadata, in.format, in.audio.FileName)
	if rel != oldRel {
		var err error
		if rel, err = library.FreePath(in.dir, rel); err != nil {
			return nil, err
		}
	}

	if err := p.saveFile(ctx, in, track, rel); err != nil {
		return nil, err
	}

	old := filepath.Join(in.dir, oldRel)
	target := filepath.Join(in.dir, rel)
	if rel != oldRel {
		// Both files exist until commit; only then the old one goes.
		if err := library.Place(in.staged, target); err != nil {
			return nil, err
		}
		in.files.OnRollback(func() { library.RemoveFile(target) })
		in.files.AfterCommit(func() { library.RemoveFile(old) })
		return &Result{Outcome: application.IngestReplaced, Path: target}, nil
	}

	// Same path: overwriting before commit would lose the old file if the
	// commit fails, so the new one waits in the scratch directory.
	pending := filepath.Join(filepath.Dir(in.staged), "pending-"+filepath.Base(in.staged))
	if err := os.Rename(in.staged, pending); err != nil {
		return nil, err
	}
	in.files.OnRollback(func() { library.RemoveFile(pending) })
	in.files.AfterCommit(func() { library.MoveFile(pending, target) })
	return &Result{Outcome: application.IngestReplaced, Path: target}, nil
}

// saveFile writes the Track's final tags into the staged file, so any
// Navidrome client shows the same metadata the database holds.
func (p *Pipeline) saveFile(ctx context.Context, in incoming, track *domain.Track, rel string) error {
	track.Path = rel
	track.Format = in.format
	track.DurationMs = in.probe.durationMs
	track.Quality = in.probe.quality

	if err := writeTags(in.staged, track.Metadata); err != nil {
		return err
	}
	return p.Tracks.SaveTrack(ctx, track)
}

// addSource keeps the Telegram file_id, which lets inline mode resend the
// track instantly even after its file was replaced.
func (p *Pipeline) addSource(ctx context.Context, in incoming, track *domain.Track) error {
	source := newSource(track, in.ref, in.audio.TelegramFile)
	if err := p.Tracks.SaveSource(ctx, source); err != nil {
		return err
	}

	return p.Uploads.Save(ctx, &domain.Upload{
		UserID:        in.job.UserID,
		TrackID:       track.ID,
		TrackSourceID: source.ID,
	})
}

// stage copies the Provider's stream into the scratch directory, so the
// following steps can read and tag a local file.
func (p *Pipeline) stage(audio *application.FetchedAudio) (string, error) {
	defer audio.Body.Close()

	dir := filepath.Join(p.MusicDir, scratchDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	file, err := os.CreateTemp(dir, "ingest-*"+audio.Format.Ext())
	if err != nil {
		return "", err
	}

	_, copyErr := io.Copy(file, audio.Body)
	closeErr := file.Close()
	if err := errors.Join(copyErr, closeErr); err != nil {
		_ = os.Remove(file.Name())
		return "", fmt.Errorf("copy audio: %w", err)
	}
	return file.Name(), nil
}

// ClearScratch drops leftovers of a crashed run; call it before workers start.
func (p *Pipeline) ClearScratch() error {
	return os.RemoveAll(filepath.Join(p.MusicDir, scratchDir))
}
