package auth

import (
	"context"
	"sync"

	"github.com/gravitl/netmaker/scope"
)

type providerRegistry struct {
	mu        sync.RWMutex
	providers map[scope.Scope]map[string]Provider
}

var registry = &providerRegistry{
	providers: make(map[scope.Scope]map[string]Provider),
}

func (r *providerRegistry) set(s scope.Scope, key string, p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.providers[s] == nil {
		r.providers[s] = make(map[string]Provider)
	}
	r.providers[s][key] = p
}

func (r *providerRegistry) get(s scope.Scope, key string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.providers[s]
	if !ok {
		return nil, false
	}
	p, ok := m[key]
	return p, ok
}

func (r *providerRegistry) delete(s scope.Scope, key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if m, ok := r.providers[s]; ok {
		delete(m, key)
	}
}

// fromContext returns the provider for the scope embedded in ctx.
func (r *providerRegistry) fromContext(ctx context.Context) (Provider, bool) {
	return r.get(scope.Level(ctx), scope.ID(ctx))
}
