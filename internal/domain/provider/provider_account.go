package provider

import (
	"bytes"
	"maps"
	"slices"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
)

// ProviderAccount is an aggregate root: a User's token for a Provider, the
// last known Collection, and what the bot has mirrored from it.
type ProviderAccount struct {
	UserID uint        `gorm:"primaryKey"`
	User   access.User `gorm:"constraint:OnDelete:CASCADE;"`

	Provider ProviderName `gorm:"primaryKey"`
	// Token is sealed with SECRET_KEY.
	Token  []byte                `gorm:"not null"`
	Status ProviderAccountStatus `gorm:"not null"`
	// InvalidatedAt is when the Provider last rejected the token.
	InvalidatedAt *time.Time `gorm:"index"`

	// Collection is the Provider Collection from the last Import or Sync;
	// nil before the first Import.
	Collection   *CollectionSnapshot `gorm:"type:jsonb;serializer:json"`
	Mirror       MirrorState         `gorm:"type:jsonb;serializer:json"`
	MirrorWanted bool                `gorm:"not null"`
	SyncedAt     *time.Time
	// SyncTriedAt is the last Sync attempt, so a failing Sync is not retried
	// before the interval passes.
	SyncTriedAt *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

// ProviderAccountStatus is a value object.
type ProviderAccountStatus string

const (
	ProviderAccountActive ProviderAccountStatus = "active"
	// ProviderAccountInvalid: the Provider rejects the token. Sync stops
	// until the user connects the account again.
	ProviderAccountInvalid ProviderAccountStatus = "invalid"
	// ProviderAccountDisconnected: the user removed the token. The
	// Collection and Mirror are kept, so reconnecting continues Sync and the
	// Mirror.
	ProviderAccountDisconnected ProviderAccountStatus = "disconnected"
)

func (a *ProviderAccount) Connect(token []byte) {
	a.Token = token
	a.Status = ProviderAccountActive
}

func (a *ProviderAccount) Disconnect() {
	a.Token = []byte{}
	a.Status = ProviderAccountDisconnected
}

func (a *ProviderAccount) SyncDue(now time.Time, interval time.Duration) bool {
	if a.Status != ProviderAccountActive || a.Collection == nil {
		return false
	}
	for _, at := range []*time.Time{a.SyncedAt, a.SyncTriedAt} {
		if at != nil && now.Sub(*at) < interval {
			return false
		}
	}
	return true
}

func (a *ProviderAccount) TrySync(at time.Time) {
	a.SyncTriedAt = &at
}

// UsableToken returns the sealed token of an active account,
// ErrAccountDisconnected or ErrTokenRejected otherwise.
func (a *ProviderAccount) UsableToken() ([]byte, error) {
	switch a.Status {
	case ProviderAccountActive:
		return a.Token, nil
	case ProviderAccountDisconnected:
		return nil, ErrAccountDisconnected
	default:
		return nil, ErrTokenRejected
	}
}

// Invalidate marks the account invalid after the Provider rejected the
// token. It does nothing if the account is no longer active or its token
// changed: the user may have sent a new one meanwhile. It returns whether
// the account changed.
func (a *ProviderAccount) Invalidate(rejected []byte, now time.Time) bool {
	if a.Status != ProviderAccountActive || !bytes.Equal(a.Token, rejected) {
		return false
	}
	a.Status = ProviderAccountInvalid
	a.InvalidatedAt = &now
	return true
}

func (a *ProviderAccount) MirrorPending() bool {
	return a.MirrorWanted && a.Collection != nil
}

// Mirrored records a Mirror built from the Collection remembered at
// basedOn. The Mirror stays wanted unless it was complete and no newer
// Collection came meanwhile.
func (a *ProviderAccount) Mirrored(state MirrorState, complete bool, basedOn *time.Time) {
	a.Mirror = state
	a.MirrorWanted = !complete || !sameTime(a.SyncedAt, basedOn)
}

func (a *ProviderAccount) Remember(collection *CollectionSnapshot, at time.Time) {
	a.Collection = collection
	a.MirrorWanted = true
	a.SyncedAt = &at
}

// WantedMirror returns the Mirror the bot should reach.
//
// found maps tracks to their Navidrome songs. coming holds tracks that will
// get a song later: still downloading or not indexed yet. A track that
// failed to download is in neither.
//
// Navidrome sorts stars by the time they were set. So no new stars are set
// while any liked track is coming: otherwise the order would follow the
// downloads, not the likes.
//
// complete is false while any track is coming.
func (a *ProviderAccount) WantedMirror(found map[string]string, coming map[string]bool) (wanted WantedMirror, complete bool) {
	c := a.Collection
	wanted = WantedMirror{
		MirrorState: MirrorState{Starred: map[string]string{}, Playlists: maps.Clone(a.Mirror.Playlists)},
		playlists:   c.Playlists,
	}
	if wanted.Playlists == nil {
		wanted.Playlists = map[string]MirroredPlaylist{}
	}

	likedComing := false
	for _, ref := range c.Liked {
		likedComing = likedComing || coming[ref]
		if song, ok := a.Mirror.Starred[ref]; ok {
			wanted.Starred[ref] = song
		}
	}
	complete = !likedComing
	if !likedComing {
		for _, ref := range c.Liked {
			if song := found[ref]; song != "" {
				wanted.Starred[ref] = song
			}
		}
	}
	wanted.starOrder = slices.Clone(c.Liked)
	slices.Reverse(wanted.starOrder)

	for _, playlist := range c.Playlists {
		var songs []string
		for _, ref := range playlist.Tracks {
			complete = complete && !coming[ref]
			if song := found[ref]; song != "" {
				songs = append(songs, song)
			}
		}
		mirrored, exists := a.Mirror.Playlists[playlist.ID]
		if !exists && len(songs) == 0 {
			continue
		}
		wanted.Playlists[playlist.ID] = MirroredPlaylist{NavidromeID: mirrored.NavidromeID, Songs: songs}
	}
	return wanted, complete
}

func sameTime(a, b *time.Time) bool {
	return a == nil && b == nil || a != nil && b != nil && a.Equal(*b)
}
