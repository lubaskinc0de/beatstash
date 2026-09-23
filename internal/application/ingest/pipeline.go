package ingest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/lubaskinc0de/navidrome-tg/internal/application"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

// scratchDir lives inside the Library so the final rename is atomic;
// Navidrome skips hidden directories.
const scratchDir = ".navidrome-tg"

// Tags of the same recording from different sources may round its length
// differently; a bigger gap means another edit or version.
const duplicateToleranceMs = 2000

type Pipeline struct {
	providers *application.Providers
	tracks    application.TrackRepository
	uploads   application.UploadRepository
	lock      application.LibraryLock
	library   string
}

func NewPipeline(
	providers *application.Providers,
	tracks application.TrackRepository,
	uploads application.UploadRepository,
	lock application.LibraryLock,
	library string,
) *Pipeline {
	return &Pipeline{providers: providers, tracks: tracks, uploads: uploads, lock: lock, library: library}
}

// Result is what Process did. The file changes are already on disk, so the
// caller runs Commit once the transaction commits and Rollback otherwise.
type Result struct {
	Outcome application.IngestOutcome
	Path    string

	Commit   func()
	Rollback func()
}

func noop() {}

// incoming carries what Process learned about the audio down to the store step.
type incoming struct {
	job    *domain.IngestJob
	ref    domain.TrackRef
	audio  *application.FetchedAudio
	format domain.Format
	probe  *probe
	meta   domain.Metadata
	staged string
}

// Process turns the job's audio into a Track. Database writes go through
// ctx's transaction; the file is moved into the Library last.
func (p *Pipeline) Process(ctx context.Context, job *domain.IngestJob) (*Result, error) {
	ref := job.Ref()

	// A forwarded copy of a known file needs no download at all.
	if result, err := p.knownSource(ctx, job, ref); result != nil || err != nil {
		return result, wrapStep(stepSource, err)
	}

	// A Provider without the Capability will not grow it on retry.
	fetcher, err := p.providers.Fetcher(ref.Provider)
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
	// path choice run one at a time: otherwise two workers could both miss
	// the duplicate and store the same track twice. The lock is held until
	// the transaction ends.
	if err := p.lock.Lock(ctx); err != nil {
		return nil, wrapStep(stepStore, err)
	}
	// Another job may have added this source while we were fetching.
	if result, err := p.knownSource(ctx, job, ref); result != nil || err != nil {
		return result, wrapStep(stepSource, err)
	}

	in := incoming{job: job, ref: ref, audio: audio, format: format, probe: probe, meta: meta, staged: staged}

	// Inbox tracks lack the fields a Duplicate is matched by.
	if meta.Complete() {
		duplicate, err := p.tracks.FindDuplicate(ctx, meta, probe.durationMs, duplicateToleranceMs)
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
	releaser, err := p.providers.Releaser(job.Provider)
	if errors.Is(err, application.ErrCapabilityNotSupported) {
		return nil
	}
	if err != nil {
		return err
	}
	return releaser.Release(ctx, job.Ref())
}

// knownSource records the Upload for an already stored Track Ref, or
// returns nil when the ref is new.
func (p *Pipeline) knownSource(ctx context.Context, job *domain.IngestJob, ref domain.TrackRef) (*Result, error) {
	source, err := p.tracks.FindSource(ctx, ref.Provider, ref.ID)
	if errors.Is(err, application.ErrTrackNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	err = p.uploads.Save(ctx, &domain.Upload{
		UserID:        job.UserID,
		TrackID:       source.TrackID,
		TrackSourceID: source.ID,
	})
	if err != nil {
		return nil, err
	}
	return &Result{Outcome: application.IngestAlreadyExists, Commit: noop, Rollback: noop}, nil
}

func (p *Pipeline) storeNew(ctx context.Context, in incoming) (*Result, error) {
	rel, err := freePath(p.library, layoutPath(in.meta, in.format, in.audio.FileName))
	if err != nil {
		return nil, err
	}

	// Database rows first, the file last: until the transaction commits
	// the rows are invisible, and Rollback takes the file back out.
	track := &domain.Track{Metadata: in.meta}
	if err := p.saveFile(ctx, in, track, rel); err != nil {
		return nil, err
	}
	if err := p.addSource(ctx, in, track); err != nil {
		return nil, err
	}

	target, err := p.place(in.staged, rel)
	if err != nil {
		return nil, err
	}

	outcome := application.IngestStored
	if !in.meta.Complete() {
		outcome = application.IngestStoredInInbox
	}
	return &Result{
		Outcome:  outcome,
		Path:     target,
		Commit:   noop,
		Rollback: func() { os.Remove(target) },
	}, nil
}

// mergeDuplicate always keeps the new source and Upload, and swaps the file
// only for better Quality. The Track keeps its metadata, so the path stays
// the same apart from the extension.
func (p *Pipeline) mergeDuplicate(ctx context.Context, in incoming, track *domain.Track) (*Result, error) {
	if err := p.addSource(ctx, in, track); err != nil {
		return nil, err
	}
	if !in.probe.quality.Better(track.Quality) {
		return &Result{Outcome: application.IngestAlreadyExists, Commit: noop, Rollback: noop}, nil
	}

	oldRel := track.Path
	rel := layoutPath(track.Metadata, in.format, in.audio.FileName)
	if rel != oldRel {
		var err error
		if rel, err = freePath(p.library, rel); err != nil {
			return nil, err
		}
	}

	if err := p.saveFile(ctx, in, track, rel); err != nil {
		return nil, err
	}

	old := filepath.Join(p.library, oldRel)
	target := filepath.Join(p.library, rel)
	if rel != oldRel {
		// Both files exist until commit; only then the old one goes.
		if _, err := p.place(in.staged, rel); err != nil {
			return nil, err
		}
		return &Result{
			Outcome:  application.IngestReplaced,
			Path:     target,
			Commit:   func() { os.Remove(old) },
			Rollback: func() { os.Remove(target) },
		}, nil
	}

	// Same path: overwriting before commit would lose the old file if the
	// commit fails, so the new one waits in the scratch directory.
	pending := filepath.Join(filepath.Dir(in.staged), "pending-"+filepath.Base(in.staged))
	if err := os.Rename(in.staged, pending); err != nil {
		return nil, err
	}
	return &Result{
		Outcome:  application.IngestReplaced,
		Path:     target,
		Commit:   func() { os.Rename(pending, target) },
		Rollback: func() { os.Remove(pending) },
	}, nil
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
	return p.tracks.SaveTrack(ctx, track)
}

// addSource keeps the Telegram file_id, which lets inline mode resend the
// track instantly even after its file was replaced.
func (p *Pipeline) addSource(ctx context.Context, in incoming, track *domain.Track) error {
	source := &domain.TrackSource{TrackID: track.ID, Provider: in.ref.Provider, Ref: in.ref.ID}
	if file := in.audio.TelegramFile; file != nil {
		source.TelegramFileID = file.ID
		source.TelegramFileKind = file.Kind
	}
	if err := p.tracks.SaveSource(ctx, source); err != nil {
		return err
	}

	return p.uploads.Save(ctx, &domain.Upload{
		UserID:        in.job.UserID,
		TrackID:       track.ID,
		TrackSourceID: source.ID,
	})
}

// place moves the staged file into the Library in one atomic rename, so
// Navidrome never sees a half-written file.
func (p *Pipeline) place(staged, rel string) (string, error) {
	target := filepath.Join(p.library, rel)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(staged, target); err != nil {
		return "", err
	}
	return target, nil
}

// stage copies the Provider's stream into the scratch directory, so the
// following steps can read and tag a local file.
func (p *Pipeline) stage(audio *application.FetchedAudio) (string, error) {
	defer audio.Body.Close()

	dir := filepath.Join(p.library, scratchDir)
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
		os.Remove(file.Name())
		return "", fmt.Errorf("copy audio: %w", err)
	}
	return file.Name(), nil
}

// ClearScratch drops leftovers of a crashed run; call it before workers start.
func (p *Pipeline) ClearScratch() error {
	return os.RemoveAll(filepath.Join(p.library, scratchDir))
}
