package providers

import (
	"fmt"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
)

type Registry struct {
	byName map[provider.ProviderName]Provider
}

func (r *Registry) Fetcher(name provider.ProviderName) (Fetcher, error) {
	return capability[Fetcher](r, name)
}

func (r *Registry) Releaser(name provider.ProviderName) (Releaser, error) {
	return capability[Releaser](r, name)
}

func (r *Registry) Recognizer(name provider.ProviderName) (Recognizer, error) {
	return capability[Recognizer](r, name)
}

func (r *Registry) TokenChecker(name provider.ProviderName) (TokenChecker, error) {
	return capability[TokenChecker](r, name)
}

func (r *Registry) CollectionLister(name provider.ProviderName) (CollectionLister, error) {
	return capability[CollectionLister](r, name)
}

func NewRegistry(providers ...Provider) *Registry {
	byName := make(map[provider.ProviderName]Provider, len(providers))
	for _, p := range providers {
		byName[p.Name()] = p
	}
	return &Registry{byName: byName}
}

func capability[C any](r *Registry, name provider.ProviderName) (C, error) {
	c, ok := r.byName[name].(C)
	if !ok {
		var zero C
		return zero, fmt.Errorf("%w: %s has no %T", ErrCapabilityNotSupported, name, (*C)(nil))
	}
	return c, nil
}
