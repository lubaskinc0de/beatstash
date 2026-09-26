// Package provider: services music comes from.
//
// Aggregate: ProviderAccount, a User's token for a Provider, the last known
// Collection, and what the bot has mirrored from it into Navidrome.
//
// Value objects: ProviderName, TrackRef, CollectionSnapshot, MirrorState,
// WantedMirror, MirrorChange.
//
// Provider capabilities (Fetcher, Recognizer, ...) are application ports,
// not part of the domain.
//
// Fields are exported only because gorm needs them. Change an aggregate
// only through its methods.
package provider
