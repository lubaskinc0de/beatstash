package import_collection

import (
	"context"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/providers"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

// Plan counts the collection and the part of it the library lacks: only
// that part is downloaded.
type Plan struct {
	Total        int
	Missing      int
	MissingBytes int64
}

// survey lists the collection and the tracks of it the library lacks.
func survey(
	ctx context.Context,
	registry *providers.Registry,
	libraries repositories.Libraries,
	tracks repositories.Tracks,
	userID uint,
	provider domain.ProviderName,
) (*providers.Collection, []providers.ListedTrack, error) {
	lister, err := registry.CollectionLister(provider)
	if err != nil {
		return nil, nil, err
	}
	collection, err := lister.Collection(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	library, err := libraries.Personal(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	all := collection.Tracks()
	known, err := tracks.KnownRefs(ctx, library.ID, provider, providers.RefIDs(all))
	if err != nil {
		return nil, nil, err
	}
	have := providers.RefSet(known)
	var missing []providers.ListedTrack
	for _, track := range all {
		if !have[track.Ref.ID] {
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
