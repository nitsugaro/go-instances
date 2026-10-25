package cacheinstance

import (
	"context"
	"sync"
)

type Registry struct {
	lifecycle gate
	mu        sync.RWMutex
	closed    bool
	loader    ConfigLoader
	kinds     map[string]*kindState
}

func NewRegistry(options CacheOptions) *Registry {
	return &Registry{loader: options.Loader, kinds: make(map[string]*kindState)}
}

func (r *Registry) Close(ctx context.Context) error {
	if err := r.lifecycle.lock(ctx, true); err != nil {
		return err
	}
	defer r.lifecycle.unlock(true)
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	r.closed = true
	r.kinds = nil
	return nil
}
