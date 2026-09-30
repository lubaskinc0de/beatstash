package import_collection

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/providers"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/ingest"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
)

// Plan counts the collection and the part of it the library lacks: only
// that part is downloaded.
type Plan struct {
	Total        int
	Missing      int
	MissingBytes int64
	// Usage is the Personal Library's. The Import starts even if the
	// missing tracks seem not to fit: their sizes are guesses.
	Usage library.Usage
	// BatchID is the batch that downloads the missing tracks; zero if none.
	BatchID uint
}

type surveyed struct {
	collection *providers.Collection
	// missing are the tracks the user lacks.
	missing  []providers.ListedTrack
	personal *library.Library
}

// survey lists the collection and the tracks of it the user lacks: neither
// their Personal Library nor an Attached Library they see has the track.
func survey(
	ctx context.Context,
	registry *providers.Registry,
	libs repositories.Libraries,
	attached *libraries.Attached,
	tracks repositories.Tracks,
	userID uint,
	providerName provider.ProviderName,
) (*surveyed, error) {
	lister, err := registry.CollectionLister(providerName)
	if err != nil {
		return nil, err
	}
	collection, err := lister.Collection(ctx, userID)
	if err != nil {
		return nil, err
	}
	personal, err := libs.Personal(ctx, userID)
	if err != nil {
		return nil, err
	}
	visible, err := attached.VisibleTo(ctx, userID)
	if err != nil {
		return nil, err
	}
	all := collection.Tracks()
	refs := make([]provider.TrackRef, 0, len(all))
	for _, track := range all {
		refs = append(refs, track.Ref)
	}
	kept := library.KeptLibraries(personal, visible)
	have, err := tracks.KnownSources(ctx, libraries.IDs(kept), refs)
	if err != nil {
		return nil, err
	}
	var missing []providers.ListedTrack
	for _, track := range all {
		if !have[track.Ref] {
			missing = append(missing, track)
		}
	}
	return &surveyed{collection: collection, missing: missing, personal: personal}, nil
}

func (s *surveyed) plan() *Plan {
	plan := &Plan{Total: len(s.collection.Tracks()), Missing: len(s.missing)}
	for _, track := range s.missing {
		plan.MissingBytes += track.Bytes
	}
	return plan
}

type ImportProgress struct {
	Provider provider.ProviderName
	Total    int
	Progress repositories.BatchProgress
}

// progressOf keeps the order of the batches.
func progressOf(ctx context.Context, queue repositories.IngestQueue, batches []ingest.IngestBatch) ([]ImportProgress, error) {
	ids := make([]uint, 0, len(batches))
	for _, batch := range batches {
		ids = append(ids, batch.ID)
	}
	progress, err := queue.BatchProgress(ctx, ids)
	if err != nil {
		return nil, err
	}
	imports := make([]ImportProgress, 0, len(batches))
	for _, batch := range batches {
		imports = append(imports, ImportProgress{Provider: batch.Provider, Total: batch.Total, Progress: progress[batch.ID]})
	}
	return imports, nil
}
