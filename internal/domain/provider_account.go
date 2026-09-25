package domain

import "time"

type ProviderAccountStatus string

const (
	ProviderAccountActive ProviderAccountStatus = "active"
	// ProviderAccountInvalid: the Provider stopped accepting the token; Sync
	// waits until the user connects the account again.
	ProviderAccountInvalid ProviderAccountStatus = "invalid"
	// ProviderAccountDisconnected: the user took the token back. What the
	// account knew of the collection stays, so a reconnection resumes Sync and
	// the Mirror.
	ProviderAccountDisconnected ProviderAccountStatus = "disconnected"
)

type ProviderAccount struct {
	UserID uint `gorm:"primaryKey"`
	User   User `gorm:"constraint:OnDelete:CASCADE;"`

	Provider ProviderName `gorm:"primaryKey"`
	// Token is sealed with SECRET_KEY.
	Token  []byte                `gorm:"not null"`
	Status ProviderAccountStatus `gorm:"not null"`

	// Collection is the Provider Collection as the last Import or Sync saw
	// it; nil until the first Import.
	Collection   *CollectionSnapshot `gorm:"type:jsonb;serializer:json"`
	Mirror       MirrorState         `gorm:"type:jsonb;serializer:json"`
	MirrorWanted bool                `gorm:"not null"`
	SyncedAt     *time.Time
	// SyncTriedAt keeps a failing Sync from asking the Provider again
	// before the interval is out.
	SyncTriedAt *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

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

func (a *ProviderAccount) Invalidate() {
	a.Status = ProviderAccountInvalid
}

func (a *ProviderAccount) MirrorPending() bool {
	return a.MirrorWanted && a.Collection != nil
}

func (a *ProviderAccount) Mirrored(state MirrorState, wanted bool) {
	a.Mirror = state
	a.MirrorWanted = wanted
}

func (a *ProviderAccount) Remember(collection *CollectionSnapshot, at time.Time) {
	a.Collection = collection
	a.MirrorWanted = true
	a.SyncedAt = &at
}
