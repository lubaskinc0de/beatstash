package enrich_tracks

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"

	"github.com/lubaskinc0de/beatstash/internal/application/common"
	"github.com/lubaskinc0de/beatstash/internal/application/common/libraries"
	"github.com/lubaskinc0de/beatstash/internal/application/common/providers"
	"github.com/lubaskinc0de/beatstash/internal/application/common/repositories"
	"github.com/lubaskinc0de/beatstash/internal/domain/library"
	"github.com/lubaskinc0de/beatstash/internal/domain/provider"
)

// FillGenresAndLabels looks up the genres and the label of the Tracks stored
// before the bot knew them: first from the Providers of their Sources, then
// from their files. A Provider acts for the owner of the Track's Library, or
// for the author of a shared Track. A Track whose Provider failed is left for
// the next run. Attached Libraries follow Navidrome instead.
type FillGenresAndLabels struct {
	Libraries repositories.Libraries
	Tracks    repositories.Tracks
	Shared    repositories.SharedTracks
	Providers *providers.Registry
	Tags      common.AudioTags
	MusicDir  string
}

func (i *FillGenresAndLabels) Execute(ctx context.Context) error {
	tracks, err := i.Tracks.Unenriched(ctx)
	if err != nil || len(tracks) == 0 {
		return err
	}
	all, err := i.Libraries.All(ctx)
	if err != nil {
		return err
	}
	libs := make(map[uint]*library.Library, len(all))
	for n := range all {
		libs[all[n].ID] = &all[n]
	}
	actors, err := i.actors(ctx, tracks, libs)
	if err != nil {
		return err
	}
	described, failed := i.describe(ctx, tracks, actors)

	var filled []library.Track
	var errs []error
	for n := range tracks {
		t := &tracks[n]
		if failed[t.ID] {
			continue
		}
		probe, err := i.Tags.Probe(filepath.Join(libraries.Dir(i.MusicDir, libs[t.LibraryID]), t.Path))
		if err != nil {
			errs = append(errs, err)
			continue
		}
		var sources []library.Metadata
		for _, source := range t.Sources {
			if d := described[source.TrackRef()]; d != nil {
				sources = append(sources, d.Metadata)
			}
		}
		t.Enrich(append(sources, probe.Tags)...)
		filled = append(filled, *t)
	}
	return errors.Join(append(errs, i.Tracks.SetGenresAndLabels(ctx, filled))...)
}

// actors maps each Track to the User its Providers act for; a Track of the
// Shared Library nobody shares has none.
func (i *FillGenresAndLabels) actors(ctx context.Context, tracks []library.Track, libs map[uint]*library.Library) (map[uint]uint, error) {
	actors := make(map[uint]uint, len(tracks))
	var shared []uint
	for _, t := range tracks {
		switch lib := libs[t.LibraryID]; {
		case lib.OwnerID != nil:
			actors[t.ID] = *lib.OwnerID
		case lib.Kind == library.LibraryShared:
			shared = append(shared, t.ID)
		}
	}
	if len(shared) == 0 {
		return actors, nil
	}
	authors, err := i.Shared.Authors(ctx, shared)
	for id, author := range authors {
		actors[id] = author
	}
	return actors, err
}

type ask struct {
	provider provider.ProviderName
	userID   uint
}

// describe asks each Provider once per User; described goes by the Tracks'
// TrackSource refs. failed holds the Tracks whose Provider did not answer.
func (i *FillGenresAndLabels) describe(
	ctx context.Context, tracks []library.Track, actors map[uint]uint,
) (described map[provider.TrackRef]*providers.Description, failed map[uint]bool) {
	refs := map[ask][]provider.TrackRef{}
	asked := map[ask][]uint{}
	for _, t := range tracks {
		userID, ok := actors[t.ID]
		if !ok {
			continue
		}
		for _, source := range t.Sources {
			a := ask{provider: source.Provider, userID: userID}
			refs[a] = append(refs[a], source.TrackRef())
			asked[a] = append(asked[a], t.ID)
		}
	}

	described = map[provider.TrackRef]*providers.Description{}
	failed = map[uint]bool{}
	for a, refs := range refs {
		// A Provider without the Capability has nothing to tell.
		describer, err := i.Providers.Describer(a.provider)
		if err != nil {
			continue
		}
		found, err := describer.Describe(ctx, a.userID, refs)
		if err != nil {
			slog.Warn("describe_tracks", "provider", a.provider, "user_id", a.userID, "error", err)
			for _, id := range asked[a] {
				failed[id] = true
			}
			continue
		}
		for id, d := range found {
			described[provider.TrackRef{Provider: a.provider, ID: id}] = d
		}
	}
	return described, failed
}
