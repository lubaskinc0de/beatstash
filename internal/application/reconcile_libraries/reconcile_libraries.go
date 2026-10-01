package reconcile_libraries

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

// ReconcileLibraries brings the Tracks of the Personal Libraries and the
// Shared Library in step with their files. Attached Libraries follow
// Navidrome instead.
type ReconcileLibraries struct {
	Tx        repositories.TxManager
	Libraries repositories.Libraries
	Lock      repositories.LibraryLock
	Tracks    repositories.Tracks
	Disk      common.Disk
	Tags      common.AudioTags
	MusicDir  string
}

func (i *ReconcileLibraries) Execute(ctx context.Context) error {
	all, err := i.Libraries.All(ctx)
	if err != nil {
		return err
	}
	var managed []*library.Library
	for n := range all {
		if !all[n].Attached() {
			managed = append(managed, &all[n])
		}
	}
	tracks, err := i.Tracks.InLibraries(ctx, libraries.IDs(managed))
	if err != nil {
		return err
	}
	var errs []error
	for _, lib := range managed {
		errs = append(errs, i.reconcile(ctx, lib, tracks[lib.ID]))
	}
	return errors.Join(errs...)
}

// reconcile walks the Library and reads the files without the lock: Ingest
// would wait for the whole walk. Under the lock it looks again only at the
// paths where the Tracks and the files disagreed.
func (i *ReconcileLibraries) reconcile(ctx context.Context, lib *library.Library, tracks []library.Track) error {
	dir := libraries.Dir(i.MusicDir, lib)
	files, err := i.Disk.ListFiles(dir)
	if err != nil {
		return err
	}
	survey := library.SurveyFiles(tracks, files)
	if survey.Empty() {
		return nil
	}
	readings := i.read(dir, survey.Unread())

	return i.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := i.Lock.Lock(ctx, lib.ID); err != nil {
			return err
		}
		tracks, err := i.Tracks.AtPaths(ctx, lib.ID, survey.Paths())
		if err != nil {
			return err
		}
		files, err := i.Disk.StatFiles(dir, survey.Paths())
		if err != nil {
			return err
		}
		plan := survey.Reconcile(lib, tracks, files, readings)
		for _, stray := range plan.Strays {
			slog.Warn("file_put_into_shared_library", "path", filepath.Join(dir, stray.Path))
		}
		if plan.Empty() {
			return nil
		}
		if err := libraries.ReplaceTracks(ctx, i.Tracks, plan.Save, plan.Delete); err != nil {
			return err
		}
		slog.Info("library_reconciled", "library_id", lib.ID, "saved", len(plan.Save), "deleted", len(plan.Delete))
		return nil
	})
}

// read skips the files it cannot read: they wait for the next run.
func (i *ReconcileLibraries) read(dir string, files []library.LibraryFile) []library.Reading {
	readings := make([]library.Reading, 0, len(files))
	for _, file := range files {
		probe, err := i.Tags.Probe(filepath.Join(dir, file.Path))
		if err != nil {
			slog.Warn("unreadable_library_file", "path", filepath.Join(dir, file.Path), "error", err)
			continue
		}
		readings = append(readings, library.Reading{File: file, Probe: *probe})
	}
	return readings
}
