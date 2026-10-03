package sync_collection

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/accounts"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/navidrome"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/providers"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/quotas"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/ingest"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
)

// SyncCollection skips accounts with a running Import and rejected ones. Running is rechecked under the account's
// lock: an Import may start while the collection is listed. Besides the new tracks it brings the ones that once
// did not fit the Quota, so a raised Quota takes effect without a new Import.
type SyncCollection struct {
	Tx        repositories.TxManager
	Accounts  repositories.ProviderAccounts
	Providers *providers.Registry
	Queue     repositories.IngestQueue
	Batches   repositories.IngestBatches
	Waker     common.Waker
	Interval  time.Duration
	Clock     func() time.Time

	Libraries         *libraries.Libraries
	LibraryRepo       repositories.Libraries
	Tracks            repositories.Tracks
	Quotas            *quotas.Quotas
	Navidrome         navidrome.Client
	NavidromeAccounts *accounts.Navidrome
}

func (i *SyncCollection) Execute(ctx context.Context, providerName provider.ProviderName) {
	i.syncAll(ctx, providerName)
	i.mirrorAll(ctx, providerName)
}

func (i *SyncCollection) syncAll(ctx context.Context, providerName provider.ProviderName) {
	accounts, err := i.Accounts.All(ctx, providerName)
	if err != nil {
		slog.Error("list_provider_accounts", "error", err)
		return
	}
	for n := range accounts {
		account := &accounts[n]
		if err := i.syncIfDue(ctx, account); err != nil && ctx.Err() == nil {
			slog.Error("sync_collection", "user_id", account.UserID, "provider", providerName, "error", err)
		}
	}
}

func (i *SyncCollection) syncIfDue(ctx context.Context, account *provider.ProviderAccount) error {
	if !account.SyncDue(i.Clock(), i.Interval) {
		return nil
	}
	running, err := i.Batches.Running(ctx, account.UserID, account.Provider, ingest.IngestBatchImport)
	if err != nil || running {
		return err
	}
	if err := i.try(ctx, account); err != nil {
		return err
	}

	lister, err := i.Providers.CollectionLister(account.Provider)
	if err != nil {
		return err
	}
	collection, err := lister.Collection(ctx, account.UserID)
	if errors.Is(err, providers.ErrUnauthorized) {
		return i.invalidate(ctx, account)
	}
	if err != nil {
		return err
	}

	err = i.Tx.WithinTx(ctx, func(ctx context.Context) error {
		return i.remember(ctx, account.UserID, account.Provider, collection)
	})
	if err != nil {
		return err
	}
	i.Waker.Wake()
	return nil
}

func (i *SyncCollection) try(ctx context.Context, account *provider.ProviderAccount) error {
	return i.Tx.WithinTx(ctx, func(ctx context.Context) error {
		locked, err := i.Accounts.GetForUpdate(ctx, account.UserID, account.Provider)
		if err != nil {
			return err
		}
		locked.TrySync(i.Clock())
		return i.Accounts.Save(ctx, locked)
	})
}

func (i *SyncCollection) remember(ctx context.Context, userID uint, providerName provider.ProviderName, collection *providers.Collection) error {
	account, err := accounts.LockIdle(ctx, i.Accounts, i.Batches, userID, providerName)
	if errors.Is(err, accounts.ErrBatchRunning) {
		return nil
	}
	if err != nil {
		return err
	}
	snapshot := collection.Snapshot()
	added := providers.RefSet(snapshot.Added(account.Collection))
	var fresh []providers.ListedTrack
	for _, track := range collection.Tracks() {
		if added[track.Ref.ID] {
			fresh = append(fresh, track)
		}
	}
	refitted, err := i.refitted(ctx, userID, providerName, collection, added)
	if err != nil {
		return err
	}
	var jobs []*ingest.IngestJob
	for _, track := range append(fresh, refitted...) {
		jobs = append(jobs, ingest.NewJob(userID, track.Ref, track.DisplayName, time.Now()))
	}
	if err := i.Queue.Enqueue(ctx, jobs...); err != nil {
		return err
	}

	account.Remember(snapshot, i.Clock())
	return i.Accounts.Save(ctx, account)
}

// refitted picks the tracks of the collection that once did not fit the
// Quota and now fit what the Personal Library has left, by the Provider's
// guess of their size: a full Library would fetch them only to refuse them.
func (i *SyncCollection) refitted(
	ctx context.Context,
	userID uint,
	providerName provider.ProviderName,
	collection *providers.Collection,
	added map[string]bool,
) ([]providers.ListedTrack, error) {
	failures, err := i.Queue.LatestFailures(ctx, userID, providerName)
	if err != nil || len(failures) == 0 {
		return nil, err
	}
	var candidates []providers.ListedTrack
	var candidateRefs []provider.TrackRef
	for _, track := range collection.Tracks() {
		if failures[track.Ref.ID].AwaitsRoom() && !added[track.Ref.ID] {
			candidates = append(candidates, track)
			candidateRefs = append(candidateRefs, track.Ref)
		}
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	personal, err := i.LibraryRepo.Personal(ctx, userID)
	if err != nil {
		return nil, err
	}
	stored, err := i.Tracks.KnownSources(ctx, []uint{personal.ID}, candidateRefs)
	if err != nil {
		return nil, err
	}
	usage, err := i.Quotas.UsageOf(ctx, personal)
	if err != nil {
		return nil, err
	}
	var missing []providers.ListedTrack
	var sizes []int64
	for _, track := range candidates {
		if !stored[track.Ref] {
			missing = append(missing, track)
			sizes = append(sizes, track.Bytes)
		}
	}
	return missing[:usage.Fit(sizes)], nil
}

func (i *SyncCollection) invalidate(ctx context.Context, account *provider.ProviderAccount) error {
	return i.Tx.WithinTx(ctx, func(ctx context.Context) error {
		locked, err := i.Accounts.GetForUpdate(ctx, account.UserID, account.Provider)
		if err != nil {
			return err
		}
		if !locked.Invalidate(account.Token, i.Clock()) {
			return nil
		}
		return i.Accounts.Save(ctx, locked)
	})
}

// mirrorAll carries the collections into Navidrome: likes become stars and
// playlists become the user's playlists. A song id comes from the path of
// the Track's file, so a track Navidrome has not indexed yet waits for the
// next run.
func (i *SyncCollection) mirrorAll(ctx context.Context, providerName provider.ProviderName) {
	accounts, err := i.Accounts.All(ctx, providerName)
	if err != nil {
		slog.Error("list_provider_accounts", "error", err)
		return
	}
	for n := range accounts {
		account := &accounts[n]
		if !account.MirrorPending() {
			continue
		}
		if err := i.mirror(ctx, account); err != nil && ctx.Err() == nil {
			slog.Error("mirror_collection", "user_id", account.UserID, "provider", providerName, "error", err)
		}
	}
}

// mirror talks to Navidrome without holding the account, so an Import or a
// Sync never waits for it, and saves what it did even if it stopped halfway.
func (i *SyncCollection) mirror(ctx context.Context, account *provider.ProviderAccount) error {
	creds, err := i.NavidromeAccounts.Credentials(ctx, account.UserID)
	if errors.Is(err, repositories.ErrNavidromeAccountNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	songs, err := i.songs(ctx, account, creds)
	if err != nil || songs == nil {
		return err
	}

	wanted, complete := account.WantedMirror(songs.found, songs.coming)
	done, err := i.apply(ctx, creds, wanted.ChangesFrom(account.Mirror))
	return errors.Join(err, i.save(ctx, account, account.Mirror.Apply(done), err == nil && complete))
}

// apply stops at the first error; done is what went through.
func (i *SyncCollection) apply(ctx context.Context, creds navidrome.Credentials, change provider.MirrorChange) (done provider.MirrorChange, err error) {
	for _, star := range change.Star {
		if err := i.Navidrome.Star(ctx, creds, []string{star.Song}); err != nil {
			return done, err
		}
		done.Star = append(done.Star, star)
	}

	unstarred := make([]string, 0, len(change.Unstar))
	for _, star := range change.Unstar {
		unstarred = append(unstarred, star.Song)
	}
	if err := i.Navidrome.Unstar(ctx, creds, unstarred); err != nil {
		return done, err
	}
	done.Unstar = change.Unstar

	for _, playlist := range change.Playlists {
		id, err := i.Navidrome.SavePlaylist(ctx, creds, playlist.NavidromeID, playlist.Title, playlist.Songs)
		if err != nil {
			return done, err
		}
		playlist.NavidromeID = id
		done.Playlists = append(done.Playlists, playlist)
	}
	return done, nil
}

func (i *SyncCollection) save(ctx context.Context, mirrored *provider.ProviderAccount, state provider.MirrorState, complete bool) error {
	return i.Tx.WithinTx(ctx, func(ctx context.Context) error {
		account, err := i.Accounts.GetForUpdate(ctx, mirrored.UserID, mirrored.Provider)
		if errors.Is(err, repositories.ErrProviderAccountNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		account.Mirrored(state, complete, mirrored.SyncedAt)
		return i.Accounts.Save(ctx, account)
	})
}

type songLookup struct {
	found  map[string]string
	coming map[string]bool
}

// songs finds the Navidrome songs of the Collection's tracks by the paths of
// their Tracks' files; nil until the Personal Library is in Navidrome. A
// track kept in an Attached Library is its song.
func (i *SyncCollection) songs(
	ctx context.Context,
	account *provider.ProviderAccount,
	creds navidrome.Credentials,
) (*songLookup, error) {
	libs, err := i.Libraries.Of(ctx, account.UserID)
	if err != nil {
		return nil, err
	}
	if libs.Personal.NavidromeID == 0 {
		return nil, nil
	}
	indexed, err := i.Navidrome.Songs(ctx, creds, libs.Personal.NavidromeID)
	if err != nil {
		return nil, err
	}
	refs := slices.Clone(account.Collection.Liked)
	for _, playlist := range account.Collection.Playlists {
		refs = append(refs, playlist.Tracks...)
	}
	paths, err := i.Tracks.SourcePaths(ctx, libs.Personal.ID, account.Provider, refs)
	if err != nil {
		return nil, err
	}
	pending, err := i.Queue.PendingRefs(ctx, account.UserID, account.Provider)
	if err != nil {
		return nil, err
	}
	queued := providers.RefSet(pending)
	kept, err := i.keptSongs(ctx, account, libs.Attached, refs)
	if err != nil {
		return nil, err
	}

	s := &songLookup{found: map[string]string{}, coming: map[string]bool{}}
	for _, ref := range refs {
		if song, ok := kept[ref]; ok {
			s.found[ref] = song
			continue
		}
		path, stored := paths[ref]
		song, ok := indexed[path]
		switch {
		case stored && ok:
			s.found[ref] = song
		case stored || queued[ref]:
			s.coming[ref] = true
		}
	}
	return s, nil
}

func (i *SyncCollection) keptSongs(
	ctx context.Context, account *provider.ProviderAccount, attached []*library.Library, refs []string,
) (map[string]string, error) {
	if len(attached) == 0 {
		return nil, nil
	}
	return i.Tracks.SourceSongs(ctx, libraries.IDs(attached), account.Provider, refs)
}
