package import_collection

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/providers"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
)

// Plan counts the collection and the part of it the library lacks: only
// that part is downloaded.
type Plan struct {
	Total        int
	Missing      int
	MissingBytes int64
	// BatchID is the batch that downloads the missing tracks; zero if none.
	BatchID uint
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
) (*providers.Collection, []providers.ListedTrack, error) {
	lister, err := registry.CollectionLister(providerName)
	if err != nil {
		return nil, nil, err
	}
	collection, err := lister.Collection(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	personal, err := libs.Personal(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	visible, err := attached.VisibleTo(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	all := collection.Tracks()
	refs := make([]provider.TrackRef, 0, len(all))
	for _, track := range all {
		refs = append(refs, track.Ref)
	}
	kept := library.KeptLibraries(personal, visible)
	have, err := tracks.KnownSources(ctx, libraries.IDs(kept), refs)
	if err != nil {
		return nil, nil, err
	}
	var missing []providers.ListedTrack
	for _, track := range all {
		if !have[track.Ref] {
			missing = append(missing, track)
		}
	}
	return collection, missing, nil
}

func planOf(all, missing []providers.ListedTrack) *Plan {
	plan := &Plan{Total: len(all), Missing: len(missing)}
	for _, track := range missing {
		plan.MissingBytes += track.Bytes
	}
	return plan
}
