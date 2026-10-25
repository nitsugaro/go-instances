package cacheinstance

import (
	"context"
	"fmt"
)

func (r *Registry) Register(ctx context.Context, kind string, factory Factory) error {
	if err := r.enter(ctx); err != nil {
		return err
	}
	defer r.lifecycle.unlock(false)
	kind, err := clean(kind)
	if err != nil || factory == nil {
		return ErrInvalidArgument
	}
	k, err := r.kindEnter(ctx, kind, true, true)
	if err != nil {
		return err
	}
	defer k.gate.unlock(true)
	r.mu.RLock()
	registered := k.factory != nil
	r.mu.RUnlock()
	if registered {
		return ErrKindExists
	}
	candidates, err := r.loadKind(ctx, kind, factory)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err = ctx.Err(); err != nil {
		return err
	}
	for id := range candidates {
		if e := k.entries[id]; e != nil && e.value != nil {
			return fmt.Errorf("%s/%s: %w", kind, id, ErrInstanceExists)
		}
	}
	for id, value := range candidates {
		e := k.entries[id]
		if e == nil {
			e = &entry{changed: make(chan struct{})}
			k.entries[id] = e
		}
		e.value = value
		e.source = "factory"
	}
	k.factory = factory
	return nil
}

func (r *Registry) loadKind(ctx context.Context, kind string, factory Factory) (map[string]any, error) {
	candidates := make(map[string]any)
	if r.loader == nil {
		return candidates, nil
	}
	configs, err := r.loader(ctx, kind)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", kind, err)
	}
	for _, config := range configs {
		id, err := clean(config.ID)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", kind, err)
		}
		if _, exists := candidates[id]; exists {
			return nil, fmt.Errorf("%s/%s: %w", kind, id, ErrInstanceExists)
		}
		value, err := build(ctx, factory, kind, id, config.Config)
		if err != nil {
			return nil, err
		}
		candidates[id] = value
	}
	return candidates, nil
}
