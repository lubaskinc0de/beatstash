package providers

import (
	"fmt"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

type Registry struct {
	byName map[domain.ProviderName]Provider
}

func (r *Registry) Fetcher(name domain.ProviderName) (Fetcher, error) {
	return capability[Fetcher](r, name)
}

func (r *Registry) Releaser(name domain.ProviderName) (Releaser, error) {
	return capability[Releaser](r, name)
}

func (r *Registry) TokenChecker(name domain.ProviderName) (TokenChecker, error) {
	return capability[TokenChecker](r, name)
}

func (r *Registry) CollectionLister(name domain.ProviderName) (CollectionLister, error) {
	return capability[CollectionLister](r, name)
}

func NewRegistry(providers ...Provider) *Registry {
	byName := make(map[domain.ProviderName]Provider, len(providers))
	for _, p := range providers {
		byName[p.Name()] = p
	}
	return &Registry{byName: byName}
}

func capability[C any](r *Registry, name domain.ProviderName) (C, error) {
	c, ok := r.byName[name].(C)
	if !ok {
		var zero C
		return zero, fmt.Errorf("%w: %s has no %T", ErrCapabilityNotSupported, name, (*C)(nil))
	}
	return c, nil
}
