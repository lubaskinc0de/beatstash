package application

import (
	"context"
	"fmt"
	"io"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

// Provider is an external source of music. What it can do is expressed by
// the Capability interfaces it implements.
type Provider interface {
	Name() domain.ProviderName
}

type Fetcher interface {
	Fetch(ctx context.Context, ref domain.TrackRef) (*FetchedAudio, error)
}

// Releaser frees what Fetch left on the Provider side. It is called once
// the job is finished for good, so retries can fetch again.
type Releaser interface {
	Release(ctx context.Context, ref domain.TrackRef) error
}

type LinkResolver interface {
	ResolveLink(ctx context.Context, link string) ([]domain.TrackRef, error)
}

type CollectionLister interface {
	Collection(ctx context.Context, user *domain.User) ([]domain.TrackRef, error)
}

type FetchedAudio struct {
	// Body is the audio stream; the caller closes it.
	Body     io.ReadCloser
	FileName string
	Format   domain.Format

	// Hint is what the Provider knows for sure, it beats the file's tags.
	Hint domain.Metadata
	// WeakHint is used only for fields the file's tags lack.
	WeakHint domain.Metadata

	TelegramFile *domain.TelegramFile
}

type Providers struct {
	byName map[domain.ProviderName]Provider
}

func NewProviders(providers ...Provider) *Providers {
	byName := make(map[domain.ProviderName]Provider, len(providers))
	for _, p := range providers {
		byName[p.Name()] = p
	}
	return &Providers{byName: byName}
}

func (r *Providers) Fetcher(name domain.ProviderName) (Fetcher, error) {
	return capability[Fetcher](r, name)
}

func (r *Providers) Releaser(name domain.ProviderName) (Releaser, error) {
	return capability[Releaser](r, name)
}

func (r *Providers) LinkResolver(name domain.ProviderName) (LinkResolver, error) {
	return capability[LinkResolver](r, name)
}

func (r *Providers) CollectionLister(name domain.ProviderName) (CollectionLister, error) {
	return capability[CollectionLister](r, name)
}

func capability[C any](r *Providers, name domain.ProviderName) (C, error) {
	c, ok := r.byName[name].(C)
	if !ok {
		var zero C
		return zero, fmt.Errorf("%w: %s has no %T", ErrCapabilityNotSupported, name, (*C)(nil))
	}
	return c, nil
}
