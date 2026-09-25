package sync_collection

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"maps"
	"slices"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/accounts"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/navidrome"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/providers"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

// SyncCollection skips accounts with a running Import and rejected ones
// (their users hear about it once). Running is rechecked under the account's
// lock: an Import may start while the collection is listed.
type SyncCollection struct {
	Tx        repositories.TxManager
	Accounts  repositories.ProviderAccounts
	Providers *providers.Registry
	Queue     repositories.IngestQueue
	BatchRepo repositories.IngestBatches
	Waker     common.Waker
	Notifier  AccountNotifier
	Interval  time.Duration

	Libraries         repositories.Libraries
	Tracks            repositories.Tracks
	Navidrome         navidrome.Client
	NavidromeAccounts *accounts.Navidrome
}

type AccountNotifier interface {
	ProviderAccountInvalid(ctx context.Context, user *domain.User, provider domain.ProviderName)
}

func (i *SyncCollection) Execute(ctx context.Context, provider domain.ProviderName) {
	i.syncAll(ctx, provider)
	i.mirrorAll(ctx, provider)
}

func (i *SyncCollection) syncAll(ctx context.Context, provider domain.ProviderName) {
	accounts, err := i.Accounts.All(ctx, provider)
	if err != nil {
		slog.Error("list_provider_accounts", "error", err)
		return
	}
	for n := range accounts {
		account := &accounts[n]
		if err := i.syncIfDue(ctx, account); err != nil && ctx.Err() == nil {
			slog.Error("sync_collection", "user_id", account.UserID, "provider", provider, "error", err)
		}
	}
}

func (i *SyncCollection) syncIfDue(ctx context.Context, account *domain.ProviderAccount) error {
	if !account.SyncDue(time.Now(), i.Interval) {
		return nil
	}
	running, err := i.BatchRepo.Running(ctx, account.UserID, account.Provider, domain.IngestBatchImport)
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

func (i *SyncCollection) try(ctx context.Context, account *domain.ProviderAccount) error {
	return i.Tx.WithinTx(ctx, func(ctx context.Context) error {
		locked, err := i.Accounts.GetForUpdate(ctx, account.UserID, account.Provider)
		if err != nil {
			return err
		}
		locked.TrySync(time.Now())
		return i.Accounts.Save(ctx, locked)
	})
}

func (i *SyncCollection) remember(ctx context.Context, userID uint, provider domain.ProviderName, collection *providers.Collection) error {
	account, err := accounts.LockIdle(ctx, i.Accounts, i.BatchRepo, userID, provider)
	if errors.Is(err, accounts.ErrBatchRunning) {
		return nil
	}
	if err != nil {
		return err
	}
	snapshot := collection.Snapshot()
	added := providers.RefSet(snapshot.Added(account.Collection))
	for _, track := range collection.Tracks() {
		if !added[track.Ref.ID] {
			continue
		}
		job := common.NewJob(userID, track, common.MessageRef{})
		if err := i.Queue.Enqueue(ctx, job); err != nil {
			return err
		}
	}

	account.Remember(snapshot, time.Now())
	return i.Accounts.Save(ctx, account)
}

// invalidate leaves alone an account whose token changed since Sync read it:
// the user may have sent a new one while the old one was being refused.
func (i *SyncCollection) invalidate(ctx context.Context, account *domain.ProviderAccount) error {
	invalidated := false
	err := i.Tx.WithinTx(ctx, func(ctx context.Context) error {
		locked, err := i.Accounts.GetForUpdate(ctx, account.UserID, account.Provider)
		if err != nil {
			return err
		}
		if locked.Status != domain.ProviderAccountActive || !bytes.Equal(locked.Token, account.Token) {
			return nil
		}
		locked.Invalidate()
		invalidated = true
		return i.Accounts.Save(ctx, locked)
	})
	if err != nil || !invalidated {
		return err
	}
	i.Notifier.ProviderAccountInvalid(ctx, &account.User, account.Provider)
	return nil
}

// mirrorAll carries the collections into Navidrome: likes become stars and
// playlists become the user's playlists. A song id comes from the path of
// the Track's file, so a track Navidrome has not indexed yet waits for the
// next run.
func (i *SyncCollection) mirrorAll(ctx context.Context, provider domain.ProviderName) {
	accounts, err := i.Accounts.All(ctx, provider)
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
			slog.Error("mirror_collection", "user_id", account.UserID, "provider", provider, "error", err)
		}
	}
}

// mirror talks to Navidrome without holding the account, so an Import or a
// Sync never waits for it, and saves what it did even if it stopped halfway.
func (i *SyncCollection) mirror(ctx context.Context, account *domain.ProviderAccount) error {
	creds, err := i.NavidromeAccounts.Credentials(ctx, account.UserID)
	if errors.Is(err, repositories.ErrNavidromeAccountNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	resolver, err := i.resolver(ctx, account, creds)
	if err != nil || resolver == nil {
		return err
	}

	err = i.star(ctx, creds, account.Collection, &account.Mirror, resolver)
	if err == nil {
		err = i.playlists(ctx, creds, account.Collection, &account.Mirror, resolver)
	}
	return errors.Join(err, i.save(ctx, account, err != nil || resolver.waiting))
}

// save keeps the mirror wanted if an Import or a Sync remembered a new
// collection meanwhile.
func (i *SyncCollection) save(ctx context.Context, mirrored *domain.ProviderAccount, wanted bool) error {
	return i.Tx.WithinTx(ctx, func(ctx context.Context) error {
		account, err := i.Accounts.GetForUpdate(ctx, mirrored.UserID, mirrored.Provider)
		if errors.Is(err, repositories.ErrProviderAccountNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		account.Mirrored(mirrored.Mirror, wanted || !sameTime(account.SyncedAt, mirrored.SyncedAt))
		return i.Accounts.Save(ctx, account)
	})
}

func sameTime(a, b *time.Time) bool {
	return a == nil && b == nil || a != nil && b != nil && a.Equal(*b)
}

func (i *SyncCollection) star(
	ctx context.Context,
	creds navidrome.Credentials,
	collection *domain.CollectionSnapshot,
	state *domain.MirrorState,
	songs *songResolver,
) error {
	if state.Starred == nil {
		state.Starred = map[string]string{}
	}

	liked := make([]string, len(collection.Liked))
	for n, ref := range collection.Liked {
		liked[n] = songs.resolve(ref)
	}

	// Navidrome sorts stars by the time they were set, and the Provider
	// lists likes newest first: stars go on one by one from the oldest like.
	// Until every liked track has its song they wait, or their order would
	// follow the downloads.
	if !songs.waiting {
		for n := len(liked) - 1; n >= 0; n-- {
			ref, song := collection.Liked[n], liked[n]
			if song == "" || state.Starred[ref] == song {
				continue
			}
			if err := i.Navidrome.Star(ctx, creds, []string{song}); err != nil {
				return err
			}
			state.Starred[ref] = song
		}
	}

	stillLiked := providers.RefSet(collection.Liked)
	unliked := map[string]string{}
	for ref, song := range state.Starred {
		if !stillLiked[ref] {
			unliked[ref] = song
		}
	}
	if err := i.Navidrome.Unstar(ctx, creds, slices.Collect(maps.Values(unliked))); err != nil {
		return err
	}
	for ref := range unliked {
		delete(state.Starred, ref)
	}
	return nil
}

func (i *SyncCollection) playlists(
	ctx context.Context,
	creds navidrome.Credentials,
	collection *domain.CollectionSnapshot,
	state *domain.MirrorState,
	songs *songResolver,
) error {
	if state.Playlists == nil {
		state.Playlists = map[string]domain.MirroredPlaylist{}
	}

	for _, playlist := range collection.Playlists {
		var ids []string
		for _, ref := range playlist.Tracks {
			song := songs.resolve(ref)
			if song != "" {
				ids = append(ids, song)
			}
		}

		mirrored, exists := state.Playlists[playlist.ID]
		if (exists && slices.Equal(mirrored.Songs, ids)) || (!exists && len(ids) == 0) {
			continue
		}
		id, err := i.Navidrome.SavePlaylist(ctx, creds, mirrored.NavidromeID, playlist.Title, ids)
		if err != nil {
			return err
		}
		state.Playlists[playlist.ID] = domain.MirroredPlaylist{NavidromeID: id, Songs: ids}
	}
	return nil
}

func (i *SyncCollection) resolver(
	ctx context.Context,
	account *domain.ProviderAccount,
	creds navidrome.Credentials,
) (*songResolver, error) {
	library, err := i.Libraries.Personal(ctx, account.UserID)
	if err != nil {
		return nil, err
	}
	if library.NavidromeID == 0 {
		return nil, nil
	}
	songs, err := i.Navidrome.Songs(ctx, creds, library.NavidromeID)
	if err != nil {
		return nil, err
	}
	refs := slices.Clone(account.Collection.Liked)
	for _, playlist := range account.Collection.Playlists {
		refs = append(refs, playlist.Tracks...)
	}
	paths, err := i.Tracks.SourcePaths(ctx, library.ID, account.Provider, refs)
	if err != nil {
		return nil, err
	}
	pending, err := i.Queue.PendingRefs(ctx, account.UserID, account.Provider)
	if err != nil {
		return nil, err
	}
	return &songResolver{paths: paths, songs: songs, pending: providers.RefSet(pending)}, nil
}

type songResolver struct {
	paths   map[string]string
	songs   map[string]string
	pending map[string]bool

	// waiting is set once a track turned out to be still on its way to
	// Navidrome: queued for Ingest, or stored but not indexed yet.
	waiting bool
}

// resolve returns "" for a track without a song; a track whose Ingest
// failed for good has none and never will.
func (r *songResolver) resolve(ref string) string {
	path, stored := r.paths[ref]
	if !stored {
		r.waiting = r.waiting || r.pending[ref]
		return ""
	}
	song, indexed := r.songs[path]
	r.waiting = r.waiting || !indexed
	return song
}
